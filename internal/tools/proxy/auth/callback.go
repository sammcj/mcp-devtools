package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CallbackServer handles OAuth callback redirects.
type CallbackServer struct {
	server   *http.Server
	listener net.Listener
	codeCh   chan string
	errCh    chan error

	mu              sync.RWMutex
	expectedState   string
	expectedIssuer  string
	requireIssuer   bool
	expectationsSet bool
}

// NewCallbackServer creates a new callback server.
func NewCallbackServer(port int) (*CallbackServer, error) {
	logrus.WithField("requested_port", port).Debug("auth: creating callback server")

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		logrus.WithError(err).WithField("port", port).Debug("auth: requested port unavailable, trying any port")
		// Try any available port
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			logrus.WithError(err).Error("auth: failed to create listener")
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	logrus.WithField("port", actualPort).Debug("auth: callback server listening")

	cs := &CallbackServer{
		listener: listener,
		codeCh:   make(chan string, 1),
		errCh:    make(chan error, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cs.handleCallback)
	mux.HandleFunc("/wait-for-auth", cs.handleWaitForAuth)

	cs.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return cs, nil
}

// Port returns the port the server is listening on.
func (cs *CallbackServer) Port() int {
	return cs.listener.Addr().(*net.TCPAddr).Port
}

// Start starts the callback server.
func (cs *CallbackServer) Start() {
	logrus.Debug("auth: starting callback server")
	go func() {
		if err := cs.server.Serve(cs.listener); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("auth: callback server error")
			select {
			case cs.errCh <- err:
			default:
			}
		}
	}()
}

// WaitForCode waits for the authorisation code.
func (cs *CallbackServer) WaitForCode(ctx context.Context, timeout time.Duration) (string, error) {
	logrus.WithField("timeout", timeout).Debug("auth: waiting for authorisation code")
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case code := <-cs.codeCh:
		logrus.Debug("auth: authorisation code received")
		return code, nil
	case err := <-cs.errCh:
		logrus.WithError(err).Error("auth: callback error")
		return "", err
	case <-ctx.Done():
		logrus.Warn("auth: timeout waiting for authorisation code")
		return "", ctx.Err()
	}
}

// Close stops the callback server.
func (cs *CallbackServer) Close() error {
	logrus.Debug("auth: shutting down callback server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return cs.server.Shutdown(ctx)
}

// Expect records what the authorisation response must carry to be accepted:
// the state issued with the request, and the issuer identifier if the server
// sends RFC 9207 `iss`.
func (cs *CallbackServer) Expect(state, issuer string, issuerRequired bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.expectedState = state
	cs.expectedIssuer = issuer
	cs.requireIssuer = issuerRequired
	cs.expectationsSet = true
}

// validateResponse checks the authorisation response against what was sent.
// Without the state check an attacker can deliver their own code to this
// callback and bind the upstream connection to their account; the iss check is
// RFC 9207 and rejects a code relayed from a different authorisation server.
// It runs under the write lock and clears the expectation on success, so a
// state can only be redeemed once even when two callbacks arrive at once.
func (cs *CallbackServer) validateResponse(state, iss string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.expectationsSet {
		return fmt.Errorf("callback received before the expected state was registered")
	}

	if subtle.ConstantTimeCompare([]byte(state), []byte(cs.expectedState)) != 1 {
		return fmt.Errorf("state parameter does not match the value sent with the authorisation request")
	}

	switch {
	case iss == "" && cs.requireIssuer:
		return fmt.Errorf("authorisation server advertises the iss parameter but did not send it")
	case iss != "" && cs.expectedIssuer == "":
		// Accepting an iss with nothing to compare it against would make this
		// check decorative, which is worse than not having it.
		return fmt.Errorf("authorisation response carries iss %q but no issuer identifier is known for this server; configure the issuer URL so it can be compared", iss)
	case iss != "" && iss != cs.expectedIssuer:
		return fmt.Errorf("iss parameter %q does not match the expected issuer %q", iss, cs.expectedIssuer)
	}

	cs.expectationsSet = false
	return nil
}

// ServeCallback exposes the callback handler so its validation can be tested
// without driving a browser through a real authorisation server.
func (cs *CallbackServer) ServeCallback(w http.ResponseWriter, r *http.Request) {
	cs.handleCallback(w, r)
}

func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	logrus.WithField("path", r.URL.Path).Debug("auth: callback request received")

	// Validate before looking at anything else, and drop the request without
	// touching the error channel. Anything that can reach this loopback port
	// could otherwise abort an in-flight flow just by sending rubbish; the
	// caller waits for the genuine response or times out instead.
	//
	// The cost is that an authorisation server that omits state on an error
	// response leaves the caller waiting for its timeout rather than failing
	// immediately. RFC 6749 section 4.1.2.1 requires state on error responses,
	// so that is a non-compliant server.
	if err := cs.validateResponse(r.URL.Query().Get("state"), r.URL.Query().Get("iss")); err != nil {
		logrus.WithError(err).Warn("auth: rejected callback")
		http.Error(w, "Authorisation response failed validation", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		errDesc := r.URL.Query().Get("error_description")
		if errMsg != "" {
			logrus.WithFields(logrus.Fields{
				"error":       errMsg,
				"description": errDesc,
			}).Error("auth: authorisation error from server")
			http.Error(w, fmt.Sprintf("Authorisation error: %s - %s", errMsg, errDesc), http.StatusBadRequest)
			select {
			case cs.errCh <- fmt.Errorf("authorisation error: %s - %s", errMsg, errDesc):
			default:
			}
			return
		}
		logrus.Warn("auth: callback received without authorisation code")
		http.Error(w, "No authorisation code received", http.StatusBadRequest)
		return
	}

	logrus.Debug("auth: authorisation code received in callback")

	// Send code to channel
	select {
	case cs.codeCh <- code:
		logrus.Debug("auth: code sent to channel")
	default:
		logrus.Debug("auth: code channel full, discarding")
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Authorisation Successful</title></head>
<body>
<h1>Authorisation successful!</h1>
<p>You may close this window and return to the CLI.</p>
<script>window.close();</script>
</body>
</html>`)
}

func (cs *CallbackServer) handleWaitForAuth(w http.ResponseWriter, r *http.Request) {
	// Long-polling endpoint for multi-instance coordination
	select {
	case <-cs.codeCh:
		// Auth completed, but we already consumed the code
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Authentication completed")
	case <-time.After(30 * time.Second):
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, "Authentication in progress")
	case <-r.Context().Done():
		return
	}
}

// FindAvailablePort finds an available port starting from the preferred port.
func FindAvailablePort(preferred int) (int, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
	if err != nil {
		// Try any available port
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port, nil
}
