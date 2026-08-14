package quickbase

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAddReplaceDBPage(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("QUICKBASE-ACTION") != "API_AddReplaceDBPage" {
			t.Fatalf("unexpected action header: %q", r.Header.Get("QUICKBASE-ACTION"))
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)
		_, _ = io.WriteString(w, `<qdbapi><errcode>0</errcode><errtext>No error</errtext><pageID>42</pageID></qdbapi>`)
	}))
	defer server.Close()

	pageID, err := (Client{}).AddReplaceDBPage("db", server.URL, "u<t", "a&b", "a&b", []byte("before]]>after"))
	if err != nil {
		t.Fatal(err)
	}
	if pageID != 42 {
		t.Fatalf("pageID = %d, want 42", pageID)
	}
	for _, expected := range []string{"<pagename>a&amp;b</pagename>", "<usertoken>u&lt;t</usertoken>", "<apptoken>a&amp;b</apptoken>", "]]><![CDATA["} {
		if !strings.Contains(requestBody, expected) {
			t.Fatalf("request body missing %q: %s", expected, requestBody)
		}
	}
}

func TestAddReplaceDBPageReportsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<qdbapi><errcode>5</errcode><errtext>Denied</errtext></qdbapi>`)
	}))
	defer server.Close()

	_, err := (Client{}).AddReplaceDBPage("db", server.URL, "token", "", "page", []byte("body"))
	if err == nil || !strings.Contains(err.Error(), "Denied") {
		t.Fatalf("error = %v, want Denied", err)
	}
}

func TestNormalizeRealm(t *testing.T) {
	for input, want := range map[string]string{
		"acme":                        "https://acme.quickbase.com",
		"acme.quickbase.com":          "https://acme.quickbase.com",
		"https://acme.quickbase.com/": "https://acme.quickbase.com",
	} {
		if got := NormalizeRealm(input); got != want {
			t.Errorf("NormalizeRealm(%q) = %q, want %q", input, got, want)
		}
	}
}
