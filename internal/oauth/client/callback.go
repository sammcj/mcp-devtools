package client

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LocalCallbackServer implements CallbackServer for localhost OAuth redirects
type LocalCallbackServer struct {
	server      *http.Server
	listener    net.Listener
	redirectURI string
	authCodeCh  chan string
	errorCh     chan error
	logger      *logrus.Logger
	mutex       sync.RWMutex
	started     bool
}

// NewCallbackServer creates a new OAuth callback server
func NewCallbackServer(logger *logrus.Logger) CallbackServer {
	return &LocalCallbackServer{
		authCodeCh: make(chan string, 1),
		errorCh:    make(chan error, 1),
		logger:     logger,
	}
}

// Start starts the callback server on the specified port (0 for random)
func (s *LocalCallbackServer) Start(ctx context.Context, port int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.started {
		return fmt.Errorf("callback server is already started")
	}

	// Create listener
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.listener = listener
	actualPort := listener.Addr().(*net.TCPAddr).Port
	s.redirectURI = fmt.Sprintf("http://127.0.0.1:%d/callback", actualPort)

	// Create HTTP server with mux
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.started = true

	// Start server in goroutine
	go func() {
		s.logger.Debugf("OAuth callback server starting on %s", s.redirectURI)
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.WithError(err).Error("Callback server error")
			select {
			case s.errorCh <- err:
			default:
				// Channel is full, ignore
			}
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		_ = s.Stop() // Error already logged in Stop()
	}()

	return nil
}

// Stop stops the callback server
func (s *LocalCallbackServer) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.started {
		return nil
	}

	s.started = false

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.WithError(err).Warn("Error shutting down callback server")
			return err
		}
	}

	s.logger.Debug("OAuth callback server stopped")
	return nil
}

// GetRedirectURI returns the redirect URI for this callback server
func (s *LocalCallbackServer) GetRedirectURI() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.redirectURI
}

// GetAuthorizationCode returns a channel that receives the authorization code
func (s *LocalCallbackServer) GetAuthorizationCode() <-chan string {
	return s.authCodeCh
}

// GetError returns a channel that receives errors
func (s *LocalCallbackServer) GetError() <-chan error {
	return s.errorCh
}

// handleCallback handles the OAuth callback request
func (s *LocalCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		s.writeErrorPage(w, "Method not allowed", "Only GET requests are allowed")
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	// Check for error parameter first
	if errorParam := query.Get("error"); errorParam != "" {
		errorDesc := query.Get("error_description")
		if errorDesc == "" {
			errorDesc = "OAuth authorization failed"
		}

		s.logger.Warnf("OAuth error received: %s - %s", errorParam, errorDesc)
		s.writeErrorPage(w, "Authorization Failed", errorDesc)

		select {
		case s.errorCh <- fmt.Errorf("oauth error: %s - %s", errorParam, errorDesc):
		default:
			// Channel is full, ignore
		}
		return
	}

	// Extract authorization code
	code := query.Get("code")
	if code == "" {
		s.logger.Warn("No authorization code received in callback")
		s.writeErrorPage(w, "Invalid Request", "No authorization code received")

		select {
		case s.errorCh <- fmt.Errorf("no authorization code received"):
		default:
			// Channel is full, ignore
		}
		return
	}

	// State validation would happen here in a full implementation
	// For now, we'll just log it
	state := query.Get("state")
	s.logger.Debugf("Received authorization callback with state: %s", state)

	// Send success page
	s.writeSuccessPage(w)

	// Send authorization code to channel
	select {
	case s.authCodeCh <- code:
		s.logger.Debug("Authorization code sent to channel")
	default:
		s.logger.Warn("Authorization code channel is full")
	}
}

// handleRoot handles requests to the root path
func (s *LocalCallbackServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>OAuth Callback Server</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .container { max-width: 600px; margin: 0 auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1>OAuth Callback Server</h1>
        <p>This is the OAuth callback server for MCP DevTools.</p>
        <p>If you see this page, the server is running and waiting for OAuth callbacks.</p>
    </div>
</body>
</html>`))
}

// writeSuccessPage writes a success page to the response
func (s *LocalCallbackServer) writeSuccessPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	tmpl := template.Must(template.New("success").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>Authentication Successful</title>
    <meta charset="utf-8">
    <style>
        body { 
            font-family: Arial, sans-serif; 
            text-align: center; 
            padding: 50px; 
            background-color: #f0f8ff;
        }
        .container { 
            max-width: 600px; 
            margin: 0 auto; 
            background: white; 
            padding: 40px; 
            border-radius: 8px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .success { color: #28a745; }
        .icon { font-size: 48px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">✅</div>
        <h1 class="success">Authentication Successful!</h1>
        <p>You have successfully authenticated with the OAuth provider.</p>
        <p>You can now close this browser window and return to your application.</p>
        <p><strong>MCP DevTools is now authenticated and ready to use.</strong></p>
    </div>
    <script>
        // Auto-close after 3 seconds (optional)
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>`))

	_ = tmpl.Execute(w, nil)
}

// writeErrorPage writes an error page to the response
func (s *LocalCallbackServer) writeErrorPage(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	tmpl := template.Must(template.New("error").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <meta charset="utf-8">
    <style>
        body { 
            font-family: Arial, sans-serif; 
            text-align: center; 
            padding: 50px; 
            background-color: #fff5f5;
        }
        .container { 
            max-width: 600px; 
            margin: 0 auto; 
            background: white; 
            padding: 40px; 
            border-radius: 8px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .error { color: #dc3545; }
        .icon { font-size: 48px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">❌</div>
        <h1 class="error">{{.Title}}</h1>
        <p>{{.Message}}</p>
        <p>Please try again or check your OAuth configuration.</p>
    </div>
</body>
</html>`))

	data := struct {
		Title   string
		Message string
	}{
		Title:   template.HTMLEscapeString(title),
		Message: template.HTMLEscapeString(message),
	}

	_ = tmpl.Execute(w, data)
}
