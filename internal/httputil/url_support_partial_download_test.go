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
