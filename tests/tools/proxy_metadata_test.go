package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/auth"
)

// metadataDoc writes an authorisation server metadata document claiming issuer.
func metadataDoc(t *testing.T, w http.ResponseWriter, issuer string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"issuer":                 issuer,
		"authorization_endpoint": issuer + "/authorize",
		"token_endpoint":         issuer + "/token",
	}); err != nil {
		t.Errorf("failed to encode metadata: %v", err)
	}
}

// RFC 8414 section 3.1 inserts the issuer's path into the well-known URL, so a
// document served at the bare well-known URL may only claim the bare origin.
// Accepting any issuer on the same host lets one tenant answer for another.
func TestFetchServerMetadataRejectsAnotherTenantOnTheSameHost(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadataDoc(t, w, srv.URL+"/tenant-b")
	}))
	t.Cleanup(srv.Close)

	_, err := auth.FetchServerMetadata(t.Context(), srv.URL+"/tenant-a")
	if err == nil {
		t.Fatal("accepted a document claiming a different tenant on the same host")
	}
}

func TestFetchServerMetadataAcceptsTheIssuerItsURLImplies(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only the path-form well-known URL exists here, as RFC 8414 requires
		// for an issuer carrying a path.
		if r.URL.Path != "/.well-known/oauth-authorization-server/tenant-a" {
			http.NotFound(w, r)
			return
		}
		metadataDoc(t, w, srv.URL+"/tenant-a")
	}))
	t.Cleanup(srv.Close)

	metadata, err := auth.FetchServerMetadata(t.Context(), srv.URL+"/tenant-a")
	if err != nil {
		t.Fatalf("rejected a compliant document: %v", err)
	}
	if metadata.Issuer != srv.URL+"/tenant-a" {
		t.Errorf("unexpected issuer: %q", metadata.Issuer)
	}
}

// An issuer with an empty path and one with "/" build the same well-known URL,
// so neither can be told from the other and both have to be accepted.
func TestFetchServerMetadataAcceptsRootIssuerWithTrailingSlash(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadataDoc(t, w, srv.URL+"/")
	}))
	t.Cleanup(srv.Close)

	if _, err := auth.FetchServerMetadata(t.Context(), srv.URL); err != nil {
		t.Fatalf("rejected a root issuer written with a trailing slash: %v", err)
	}
}

// Discovery builds its URL from configuration, so the document has to come from
// the host that configuration named. A redirect elsewhere sources the
// authorisation and token endpoints from a party the user never nominated.
func TestFetchServerMetadataRefusesOffOriginRedirect(t *testing.T) {
	var elsewhere *httptest.Server
	elsewhere = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadataDoc(t, w, elsewhere.URL)
	}))
	t.Cleanup(elsewhere.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/.well-known/oauth-authorization-server", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	_, err := auth.FetchServerMetadata(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("followed a redirect off the configured origin")
	}
	if strings.Contains(err.Error(), elsewhere.URL) && !strings.Contains(err.Error(), "redirect") {
		t.Errorf("error does not explain the redirect refusal: %v", err)
	}
}

// A same-origin redirect is ordinary path normalisation and must still work.
func TestFetchServerMetadataFollowsSameOriginRedirect(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			http.Redirect(w, r, "/moved", http.StatusFound)
			return
		}
		metadataDoc(t, w, srv.URL)
	}))
	t.Cleanup(srv.Close)

	if _, err := auth.FetchServerMetadata(t.Context(), srv.URL); err != nil {
		t.Fatalf("refused a same-origin redirect: %v", err)
	}
}
