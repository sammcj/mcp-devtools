package youcom

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// fakeHTTPClient records the request and returns a canned response
type fakeHTTPClient struct {
	req    *http.Request
	body   []byte
	status int
	resp   string
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.req = req
	if req.Body != nil {
		f.body, _ = io.ReadAll(req.Body)
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.resp)),
	}, nil
}

const fixture = `{"results":{"web":[{"url":"https://example.com/1","title":"Result 1","description":"Snippet 1","snippets":["more"]},{"url":"https://example.com/2","title":"Result 2","description":"","snippets":["Fallback A","Fallback B"]},{"url":"","title":"No URL"}],"news":[{"url":"https://news.example/1","title":"News 1","description":"Breaking","page_age":"2026-11-12T10:00:00"}]},"metadata":{"query":"q"}}`

func testProvider(apiKey string, fake *fakeHTTPClient) *YouComProvider {
	client := NewYouComClient(apiKey)
	client.httpClient = fake
	return &YouComProvider{client: client}
}

func quietLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

func TestYouComProvider_NotRegisteredByDefault(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	t.Setenv(KeylessEnvVar, "")

	if provider := NewYouComProvider(); provider != nil {
		t.Error("Expected nil provider when neither YDC_API_KEY nor YOUCOM_SEARCH_KEYLESS is set")
	}
}

func TestYouComProvider_KeylessOptIn(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	t.Setenv(KeylessEnvVar, "true")

	provider := NewYouComProvider()
	if provider == nil || !provider.IsAvailable() {
		t.Fatal("Expected provider to be available with YOUCOM_SEARCH_KEYLESS=true")
	}
	if provider.client.apiKey != "" {
		t.Errorf("Expected keyless client, got key %q", provider.client.apiKey)
	}
}

func TestYouComProvider_KeyedFromEnv(t *testing.T) {
	t.Setenv(APIKeyEnvVar, " test-key ")
	t.Setenv(KeylessEnvVar, "")

	provider := NewYouComProvider()
	if provider == nil || !provider.IsAvailable() {
		t.Fatal("Expected provider to be available with YDC_API_KEY set")
	}
	if provider.client.apiKey != "test-key" {
		t.Errorf("Expected trimmed key, got %q", provider.client.apiKey)
	}
}

func TestYouComProvider_NameAndTypes(t *testing.T) {
	provider := testProvider("", &fakeHTTPClient{})
	if provider.GetName() != "youcom" {
		t.Errorf("Expected provider name 'youcom', got '%s'", provider.GetName())
	}
	types := provider.GetSupportedTypes()
	if len(types) != 1 || types[0] != "web" {
		t.Errorf("Expected [web], got %v", types)
	}
}

func TestYouComProvider_KeylessWebSearch(t *testing.T) {
	fake := &fakeHTTPClient{resp: fixture}
	provider := testProvider("", fake)

	resp, err := provider.Search(context.Background(), quietLogger(), "web", map[string]any{"query": "hello world", "count": float64(5)})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if fake.req.Method != http.MethodGet {
		t.Errorf("Expected GET, got %s", fake.req.Method)
	}
	if got := fake.req.URL.Scheme + "://" + fake.req.URL.Host + fake.req.URL.Path; got != KeylessSearchURL {
		t.Errorf("Expected %s, got %s", KeylessSearchURL, got)
	}
	if q := fake.req.URL.Query(); q.Get("query") != "hello world" || q.Get("count") != "5" {
		t.Errorf("Unexpected query params: %v", q)
	}
	if fake.req.Header.Get("X-API-Key") != "" {
		t.Error("Expected no X-API-Key header on keyless requests")
	}
	if fake.req.Header.Get("User-Agent") != UserAgent {
		t.Errorf("Expected User-Agent %q, got %q", UserAgent, fake.req.Header.Get("User-Agent"))
	}

	if resp.Provider != "youcom" {
		t.Errorf("Expected provider youcom, got %s", resp.Provider)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("Expected 3 results (web + news, empty URL dropped), got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "Result 1" || resp.Results[0].Description != "Snippet 1" {
		t.Errorf("Unexpected first result: %+v", resp.Results[0])
	}
	if resp.Results[1].Description != "Fallback A Fallback B" {
		t.Errorf("Expected snippets as description fallback, got %q", resp.Results[1].Description)
	}
}

func TestYouComProvider_KeyedSearch(t *testing.T) {
	fake := &fakeHTTPClient{resp: fixture}
	provider := testProvider("test-key", fake)

	resp, err := provider.Search(context.Background(), quietLogger(), "web", map[string]any{"query": "q", "count": float64(3)})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if fake.req.Method != http.MethodPost {
		t.Errorf("Expected POST, got %s", fake.req.Method)
	}
	if fake.req.URL.String() != KeyedSearchURL {
		t.Errorf("Expected %s, got %s", KeyedSearchURL, fake.req.URL)
	}
	if fake.req.Header.Get("X-API-Key") != "test-key" {
		t.Errorf("Expected X-API-Key header, got %q", fake.req.Header.Get("X-API-Key"))
	}
	var body map[string]any
	if err := json.Unmarshal(fake.body, &body); err != nil {
		t.Fatalf("Failed to decode body: %v", err)
	}
	if body["query"] != "q" || body["count"] != float64(3) {
		t.Errorf("Unexpected body: %v", body)
	}

	if len(resp.Results) != 3 || resp.Results[2].URL != "https://news.example/1" {
		t.Fatalf("Expected the news result last, got %+v", resp.Results)
	}
	if resp.Results[2].Metadata["published"] != "2026-11-12T10:00:00" {
		t.Errorf("Expected published metadata, got %v", resp.Results[2].Metadata)
	}
}

func TestYouComProvider_InvalidCount(t *testing.T) {
	provider := testProvider("", &fakeHTTPClient{resp: fixture})
	_, err := provider.Search(context.Background(), quietLogger(), "web", map[string]any{"query": "q", "count": float64(60)})
	if err == nil || !strings.Contains(err.Error(), "count must be between 1 and 50") {
		t.Errorf("Expected count validation error, got %v", err)
	}
}

func TestYouComProvider_UnsupportedSearchType(t *testing.T) {
	provider := testProvider("", &fakeHTTPClient{resp: fixture})
	_, err := provider.Search(context.Background(), quietLogger(), "news", map[string]any{"query": "q"})
	if err == nil || err.Error() != "unsupported search type for You.com: news" {
		t.Errorf("Expected unsupported type error, got %v", err)
	}
}

func TestYouComProvider_KeylessRateLimit(t *testing.T) {
	provider := testProvider("", &fakeHTTPClient{status: http.StatusTooManyRequests, resp: "limited"})
	_, err := provider.Search(context.Background(), quietLogger(), "web", map[string]any{"query": "q"})
	if err == nil || !strings.Contains(err.Error(), "YDC_API_KEY") {
		t.Errorf("Expected rate limit error mentioning YDC_API_KEY, got %v", err)
	}
}
