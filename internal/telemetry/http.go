package telemetry

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// WrapHTTPTransport wraps an existing http.Transport with OTEL instrumentation
// This preserves any existing configuration (proxy, timeouts, etc.) whilst adding tracing
func WrapHTTPTransport(transport http.RoundTripper) http.RoundTripper {
	if !IsEnabled() {
		// Tracing disabled, return original transport
		return transport
	}

	// Wrap with OTEL HTTP instrumentation
	return otelhttp.NewTransport(transport)
}

// WrapHTTPClient wraps an existing http.Client's transport with OTEL instrumentation
// Modifies the client in-place and returns it for convenience
func WrapHTTPClient(client *http.Client) *http.Client {
	if !IsEnabled() {
		// Tracing disabled, return original client
		return client
	}

	// Get the existing transport or use default
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Wrap with OTEL instrumentation
	client.Transport = otelhttp.NewTransport(transport)

	return client
}
