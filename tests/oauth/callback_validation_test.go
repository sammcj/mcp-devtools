package oauth

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	oauthclient "github.com/sammcj/mcp-devtools/internal/oauth/client"
	proxyauth "github.com/sammcj/mcp-devtools/internal/tools/proxy/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OAuth callback endpoint listens on loopback while the browser flow is
// open. Without a state check, anything that can reach it, including a web page
// the user has open, can deliver an authorisation code of its choosing and bind
// the upstream connection to an attacker's account.
func TestProxyCallbackRejectsUnexpectedState(t *testing.T) {
	server, err := proxyauth.NewCallbackServer(0)
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	cases := map[string]struct {
		query      string
		wantAccept bool
	}{
		"matching state":  {query: "?code=abc&state=the-real-state", wantAccept: true},
		"wrong state":     {query: "?code=abc&state=guessed", wantAccept: false},
		"no state at all": {query: "?code=abc", wantAccept: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server.Expect("the-real-state", "https://issuer.example", false)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/callback"+tc.query, nil)
			server.ServeCallback(rec, req)

			if tc.wantAccept {
				assert.Equal(t, 200, rec.Code)
			} else {
				assert.Equal(t, 400, rec.Code)
			}
		})
	}
}

// RFC 9207: when the authorisation server says it sends iss, a response
// without one, or with the wrong one, is a mix-up attempt.
func TestProxyCallbackValidatesIssuer(t *testing.T) {
	cases := map[string]struct {
		issuerRequired bool
		query          string
		wantAccept     bool
	}{
		"correct issuer":       {issuerRequired: true, query: "?code=a&state=s&iss=https%3A%2F%2Fissuer.example", wantAccept: true},
		"wrong issuer":         {issuerRequired: true, query: "?code=a&state=s&iss=https%3A%2F%2Fevil.example", wantAccept: false},
		"missing but promised": {issuerRequired: true, query: "?code=a&state=s", wantAccept: false},
		"missing and optional": {issuerRequired: false, query: "?code=a&state=s", wantAccept: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server, err := proxyauth.NewCallbackServer(0)
			require.NoError(t, err)
			defer func() { _ = server.Close() }()

			server.Expect("s", "https://issuer.example", tc.issuerRequired)

			rec := httptest.NewRecorder()
			server.ServeCallback(rec, httptest.NewRequest("GET", "/callback"+tc.query, nil))

			if tc.wantAccept {
				assert.Equal(t, 200, rec.Code)
			} else {
				assert.Equal(t, 400, rec.Code)
			}
		})
	}
}

// A callback that arrives before the caller has said what to expect must be
// discarded, so that a future refactor cannot quietly drop the check.
func TestProxyCallbackRejectsBeforeExpectationsSet(t *testing.T) {
	server, err := proxyauth.NewCallbackServer(0)
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	rec := httptest.NewRecorder()
	server.ServeCallback(rec, httptest.NewRequest("GET", "/callback?code=abc&state=anything", nil))

	assert.Equal(t, 400, rec.Code)
}

// An iss with nothing to compare it against must not be waved through, or the
// RFC 9207 check would be decorative whenever discovery did not yield an
// issuer identifier.
func TestProxyCallbackRejectsIssuerWithNoExpectation(t *testing.T) {
	server, err := proxyauth.NewCallbackServer(0)
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	server.Expect("s", "", false)

	rec := httptest.NewRecorder()
	server.ServeCallback(rec, httptest.NewRequest("GET", "/callback?code=a&state=s&iss=https%3A%2F%2Fanyone.example", nil))

	assert.Equal(t, 400, rec.Code)
}

// A rejected callback must not reach the waiter at all. Returning an error to
// WaitForCode would let anything able to hit the loopback port abort an
// in-flight authorisation, which is the same denial of service the ordering
// change was meant to close.
func TestProxyCallbackRejectionDoesNotAbortTheFlow(t *testing.T) {
	server, err := proxyauth.NewCallbackServer(0)
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	server.Expect("the-real-state", "", false)

	// Two drive-by requests: one claiming an error, one with a guessed state.
	for _, query := range []string{"?error=access_denied&state=guessed", "?code=evil&state=also-guessed"} {
		rec := httptest.NewRecorder()
		server.ServeCallback(rec, httptest.NewRequest("GET", "/callback"+query, nil))
		assert.Equal(t, 400, rec.Code)
	}

	// The genuine response still arrives, and it is what the waiter receives.
	rec := httptest.NewRecorder()
	server.ServeCallback(rec, httptest.NewRequest("GET", "/callback?code=abc&state=the-real-state", nil))
	require.Equal(t, 200, rec.Code)

	code, err := server.WaitForCode(context.Background(), 2*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "abc", code)
}

// A state is good for one response. Replaying it must not deliver a second
// code into the same flow.
func TestProxyCallbackStateIsSingleUse(t *testing.T) {
	server, err := proxyauth.NewCallbackServer(0)
	require.NoError(t, err)
	defer func() { _ = server.Close() }()

	server.Expect("once", "", false)

	rec := httptest.NewRecorder()
	server.ServeCallback(rec, httptest.NewRequest("GET", "/callback?code=first&state=once", nil))
	require.Equal(t, 200, rec.Code)

	rec = httptest.NewRecorder()
	server.ServeCallback(rec, httptest.NewRequest("GET", "/callback?code=second&state=once", nil))
	assert.Equal(t, 400, rec.Code)
}

// The browser-auth callback server carries the same rules as the proxy's, so
// it needs the same coverage: the two implementations are separate code.
func TestBrowserAuthCallbackValidation(t *testing.T) {
	cases := map[string]struct {
		state          string
		issuer         string
		issuerRequired bool
		query          string
		wantAccept     bool
	}{
		"matching state":               {state: "s", query: "?code=a&state=s", wantAccept: true},
		"wrong state":                  {state: "s", query: "?code=a&state=other", wantAccept: false},
		"no state":                     {state: "s", query: "?code=a", wantAccept: false},
		"correct issuer":               {state: "s", issuer: "https://issuer.example", issuerRequired: true, query: "?code=a&state=s&iss=https%3A%2F%2Fissuer.example", wantAccept: true},
		"wrong issuer":                 {state: "s", issuer: "https://issuer.example", issuerRequired: true, query: "?code=a&state=s&iss=https%3A%2F%2Fevil.example", wantAccept: false},
		"promised issuer absent":       {state: "s", issuer: "https://issuer.example", issuerRequired: true, query: "?code=a&state=s", wantAccept: false},
		"issuer with no expectation":   {state: "s", query: "?code=a&state=s&iss=https%3A%2F%2Fanyone.example", wantAccept: false},
		"error claimed with bad state": {state: "s", query: "?error=access_denied&state=other", wantAccept: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server, ok := oauthclient.NewCallbackServer(quietLogger()).(*oauthclient.LocalCallbackServer)
			require.True(t, ok)

			server.Expect(tc.state, tc.issuer, tc.issuerRequired)

			rec := httptest.NewRecorder()
			server.ServeCallback(rec, httptest.NewRequest("GET", "/callback"+tc.query, nil))

			if tc.wantAccept {
				assert.Equal(t, 200, rec.Code)
			} else {
				assert.NotEqual(t, 200, rec.Code)
			}
		})
	}
}
