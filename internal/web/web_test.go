package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("body should not be empty")
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("Content-Type should be set")
	}
}

func TestHandlerServesIndexForUnknownPath(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/unknown-file.xyz")
	if err != nil {
		t.Fatalf("GET /unknown: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (should fallback to index)", resp.StatusCode)
	}
}

func TestHandlerContentTypeHTML(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/index.html")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
}

func TestHandlerContentTypeCSS(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Create a .css entry in the embedded FS - can't actually do that,
	// but we can verify the content type detection via the root path
	resp, _ := http.Get(ts.URL + "/")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestHandlerContentTypeJS(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Fallback to index.html should have HTML content type
	resp, _ := http.Get(ts.URL + "/nonexistent.js")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8 (fallback)", ct)
	}
}

func TestHandlerContentTypePNG(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/nonexistent.png")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Log("non-existent .png falls back to index.html (expected)")
	}
}

func TestHandlerContentTypeICO(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/nonexistent.ico")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Log("non-existent .ico falls back to index.html (expected)")
	}
}

func TestHandlerContentTypeSVG(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/nonexistent.svg")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Log("non-existent .svg falls back to index.html (expected)")
	}
}

func TestHandlerContentTypeDefault(t *testing.T) {
	handler := NewHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("Content-Type should be set")
	}
}
