// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownload(t *testing.T) {
	t.Run("writes the response body to a file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("zip-content"))
		}))
		defer server.Close()

		fn, err := download(server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(fn)

		got, err := os.ReadFile(fn)
		if err != nil {
			t.Fatalf("couldn't read downloaded file: %v", err)
		}
		if string(got) != "zip-content" {
			t.Errorf("got %q, want %q", got, "zip-content")
		}
	})

	t.Run("errors and cleans up on a non-2xx response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer server.Close()

		fn, err := download(server.URL)
		if err == nil {
			os.Remove(fn)
			t.Fatal("expected an error for a 404 response")
		}
		if fn != "" {
			t.Errorf("expected no filename on error, got %q", fn)
		}
	})
}
