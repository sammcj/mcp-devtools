package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// CallbackServer handles OAuth callback redirects.
type CallbackServer struct {
	server   *http.Server
	listener net.Listener
	codeCh   chan string
	errCh    chan error
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
			cs.errCh <- err
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

func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	logrus.WithField("path", r.URL.Path).Debug("auth: callback request received")

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
			cs.errCh <- fmt.Errorf("authorisation error: %s - %s", errMsg, errDesc)
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
