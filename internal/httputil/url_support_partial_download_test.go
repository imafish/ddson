package httputil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- CheckPartialDownloadSupport tests ---

func TestCheckPartialDownloadSupport_SupportsPartial(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !supported {
		t.Error("expected partial download to be supported")
	}
	if size != 1024 {
		t.Errorf("expected size 1024, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_NoPartial(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Accept-Ranges header
		w.Header().Set("Content-Length", "2048")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if supported {
		t.Error("expected partial download to NOT be supported")
	}
	if size != 2048 {
		t.Errorf("expected size 2048, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_WithCredentials(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "512")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "user", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth == "" {
		t.Error("expected Authorization header to be set")
	}
}

func TestCheckPartialDownloadSupport_NoCredentialsWhenEmpty(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "512")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Error("expected no Authorization header when credentials are empty")
	}
}

func TestCheckPartialDownloadSupport_EmptyURL(t *testing.T) {
	_, _, err := CheckPartialDownloadSupport("", http.DefaultClient, "", "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestCheckPartialDownloadSupport_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestCheckPartialDownloadSupport_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 404 status")
	}
}

func TestCheckPartialDownloadSupport_InvalidContentLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "not-a-number")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for invalid Content-Length")
	}
}

func TestCheckPartialDownloadSupport_StatusPartialContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer ts.Close()

	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !supported {
		t.Error("expected partial download to be supported")
	}
	if size != 4096 {
		t.Errorf("expected size 4096, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_ConnectionRefused(t *testing.T) {
	// Use a URL that will definitely fail to connect
	_, _, err := CheckPartialDownloadSupport("http://127.0.0.1:1", http.DefaultClient, "", "")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

// --- HTTPDoer interface compliance test ---

func TestHTTPDoer_HttpClientSatisfiesInterface(t *testing.T) {
	var _ HTTPDoer = http.DefaultClient
	var _ HTTPDoer = &http.Client{}
}

// --- Helper: mock HTTPDoer for edge cases ---

type mockDoer struct {
	resp *http.Response
	err  error
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestCheckPartialDownloadSupport_DoerReturnsError(t *testing.T) {
	doer := &mockDoer{resp: nil, err: fmt.Errorf("network failure")}

	_, _, err := CheckPartialDownloadSupport("http://example.com/file", doer, "", "")
	if err == nil {
		t.Fatal("expected error from doer")
	}
	if err.Error() != "network failure" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- Error Condition Tests ---

func TestCheckPartialDownloadSupport_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate authentication required
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 401 Unauthorized status")
	}
	if err.Error() != "unexpected HTTP status: 401 Unauthorized" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_AuthTokenExpired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate expired token - server returns 401
		auth := r.Header.Get("Authorization")
		if auth != "" {
			// Even with credentials, token is expired
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="The access token expired"`)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "user", "expiredtoken")
	if err == nil {
		t.Fatal("expected error for expired authentication token")
	}
	if err.Error() != "unexpected HTTP status: 401 Unauthorized" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate forbidden access
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 403 Forbidden status")
	}
	if err.Error() != "unexpected HTTP status: 403 Forbidden" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_TooManyRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate rate limiting
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 429 Too Many Requests status")
	}
	if err.Error() != "unexpected HTTP status: 429 Too Many Requests" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_BadGateway(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate proxy/gateway error
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 502 Bad Gateway status")
	}
	if err.Error() != "unexpected HTTP status: 502 Bad Gateway" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_ServiceUnavailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate server temporarily unavailable
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 503 Service Unavailable status")
	}
	if err.Error() != "unexpected HTTP status: 503 Service Unavailable" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_GatewayTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate gateway timeout
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 504 Gateway Timeout status")
	}
	if err.Error() != "unexpected HTTP status: 504 Gateway Timeout" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_Gone(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate resource no longer available
		w.WriteHeader(http.StatusGone)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for 410 Gone status")
	}
	if err.Error() != "unexpected HTTP status: 410 Gone" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_MovedPermanently(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate redirect (should not be followed by HEAD)
		w.Header().Set("Location", "http://example.com/new-location")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer ts.Close()

	// Use a client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	_, _, err := CheckPartialDownloadSupport(ts.URL, client, "", "")
	if err == nil {
		t.Fatal("expected error for 301 Moved Permanently status")
	}
	if err.Error() != "unexpected HTTP status: 301 Moved Permanently" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_MissingContentLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		// No Content-Length header
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for missing Content-Length header")
	}
}

func TestCheckPartialDownloadSupport_NegativeContentLength(t *testing.T) {
	// HTTP client typically rejects negative Content-Length headers
	// Use a mock doer to test the parsing logic directly
	doer := &mockDoer{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Accept-Ranges":  []string{"bytes"},
				"Content-Length": []string{"-100"},
			},
			Body: http.NoBody,
		},
		err: nil,
	}

	supported, size, err := CheckPartialDownloadSupport("http://example.com", doer, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size >= 0 {
		t.Errorf("expected negative size, got %d", size)
	}
	if !supported {
		t.Error("expected partial download to be supported (based on Accept-Ranges header)")
	}
}

func TestCheckPartialDownloadSupport_ZeroContentLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 0 {
		t.Errorf("expected size 0, got %d", size)
	}
	if !supported {
		t.Error("expected partial download to be supported")
	}
}

func TestCheckPartialDownloadSupport_VeryLargeContentLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		// 1 TB
		w.Header().Set("Content-Length", "1099511627776")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !supported {
		t.Error("expected partial download to be supported")
	}
	if size != 1099511627776 {
		t.Errorf("expected size 1099511627776, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_InvalidURL(t *testing.T) {
	_, _, err := CheckPartialDownloadSupport("://invalid-url", http.DefaultClient, "", "")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestCheckPartialDownloadSupport_AcceptRangesNone(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Explicitly states ranges are not accepted
		w.Header().Set("Accept-Ranges", "none")
		w.Header().Set("Content-Length", "2048")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if supported {
		t.Error("expected partial download to NOT be supported when Accept-Ranges is 'none'")
	}
	if size != 2048 {
		t.Errorf("expected size 2048, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_MultipleAcceptRangesHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Multiple Accept-Ranges headers (malformed)
		w.Header().Add("Accept-Ranges", "bytes")
		w.Header().Add("Accept-Ranges", "none")
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Go's http.Header.Get() returns the first value
	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !supported {
		t.Error("expected partial download to be supported (first Accept-Ranges header is 'bytes')")
	}
	if size != 1024 {
		t.Errorf("expected size 1024, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_WrongCredentials(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// Check for correct credentials (basic auth for "admin:secret")
		expectedAuth := "Basic YWRtaW46c2VjcmV0" // base64(admin:secret)
		if auth != expectedAuth {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Try with wrong credentials
	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "admin", "wrongpass")
	if err == nil {
		t.Fatal("expected error for wrong credentials")
	}
	if err.Error() != "unexpected HTTP status: 401 Unauthorized" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_CorrectCredentials(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// Check for correct credentials
		expectedAuth := "Basic YWRtaW46c2VjcmV0" // base64(admin:secret)
		if auth != expectedAuth {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "2048")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Try with correct credentials
	supported, size, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !supported {
		t.Error("expected partial download to be supported")
	}
	if size != 2048 {
		t.Errorf("expected size 2048, got %d", size)
	}
}

func TestCheckPartialDownloadSupport_RequestTimeout(t *testing.T) {
	// Create a mock doer that simulates timeout
	doer := &mockDoer{
		resp: nil,
		err:  fmt.Errorf("context deadline exceeded"),
	}

	_, _, err := CheckPartialDownloadSupport("http://example.com/file", doer, "", "")
	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if err.Error() != "context deadline exceeded" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckPartialDownloadSupport_DNSResolutionFailure(t *testing.T) {
	// Use a mock doer to simulate DNS failure without actual DNS lookup
	doer := &mockDoer{
		resp: nil,
		err:  fmt.Errorf("lookup host: no such host"),
	}

	_, _, err := CheckPartialDownloadSupport("http://nonexistent.invalid/file", doer, "", "")
	if err == nil {
		t.Fatal("expected error for DNS resolution failure")
	}
}

func TestCheckPartialDownloadSupport_ContentLengthOverflow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		// Value larger than int64 max
		w.Header().Set("Content-Length", "99999999999999999999")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, _, err := CheckPartialDownloadSupport(ts.URL, ts.Client(), "", "")
	if err == nil {
		t.Fatal("expected error for Content-Length overflow")
	}
}
