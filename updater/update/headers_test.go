// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/papercutsoftware/silver/updater/update"
)

func TestIsValidHeaderFieldName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple token", "X-Custom-Tenant", true},
		{"digits and dashes", "X-Tenant-42", true},
		{"empty", "", false},
		{"space", "X Custom", false},
		{"colon", "X-Custom:", false},
		{"contains CR", "X-Custom\r", false},
		{"contains LF", "X-Custom\n", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := update.IsValidHeaderFieldName(tt.input); got != tt.want {
				t.Errorf("IsValidHeaderFieldName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidHeaderFieldValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple value", "tenant-42", true},
		{"multi-value with comma", "application/json, text/plain", true},
		{"internal tab", "a\tb", true},
		{"CRLF injection", "value\r\nX-Injected: evil", false},
		{"bare CR", "value\r", false},
		{"bare LF", "value\n", false},
		{"NUL byte", "value\x00", false},
		{"DEL byte", "value\x7f", false},
		{"empty", "", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := update.IsValidHeaderFieldValue(tt.input); got != tt.want {
				t.Errorf("IsValidHeaderFieldValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateHeaders(t *testing.T) {
	t.Run("trims keys and values", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{" X-Tenant ": " tenant-42 "})
		if got["X-Tenant"] != "tenant-42" {
			t.Errorf("got %#v, want X-Tenant=tenant-42", got)
		}
	})

	t.Run("drops invalid field name", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{"X Custom": "v"})
		if len(got) != 0 {
			t.Errorf("got %#v, want empty", got)
		}
	})

	t.Run("drops invalid field value", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{"X-Header": "invalid\r\nvalue"})
		if len(got) != 0 {
			t.Errorf("got %#v, want empty", got)
		}
	})

	t.Run("drops oversized value", func(t *testing.T) {
		big := strings.Repeat("a", update.MaxHeaderValueBytes+1)
		got := update.ValidateHeaders(map[string]string{"X-Big": big})
		if len(got) != 0 {
			t.Errorf("got %#v, want empty", got)
		}
	})

	t.Run("keeps value at the size limit", func(t *testing.T) {
		fits := strings.Repeat("a", update.MaxHeaderValueBytes)
		got := update.ValidateHeaders(map[string]string{"X-Fits": fits})
		if got["X-Fits"] != fits {
			t.Errorf("expected value to survive at exactly the size limit")
		}
	})

	t.Run("drops headers matching immutable system header names case-insensitively", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{
			"user-agent":         "evil",
			"X-PROFILE-TIMEZONE": "evil",
			"X-OS-Type":          "evil",
		})
		if len(got) != 0 {
			t.Errorf("got %#v, want immutable system headers to be dropped", got)
		}
	})

	t.Run("drops hop-by-hop headers Go would ignore anyway", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{
			"Host":              "evil.example.com",
			"Content-Length":    "0",
			"Transfer-Encoding": "chunked",
			"Connection":        "close",
		})
		if len(got) != 0 {
			t.Errorf("got %#v, want hop-by-hop headers to be dropped", got)
		}
	})

	t.Run("dedupes case-variant duplicate keys deterministically", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{
			"X-Foo": "upper-value",
			"x-foo": "lower-value",
		})
		if len(got) != 1 {
			t.Fatalf("got %#v, want exactly one surviving entry", got)
		}
		if got["X-Foo"] != "upper-value" {
			t.Errorf("got %#v, want the alphabetically-first key to win", got)
		}
	})

	t.Run("allows overriding deprecated profile headers", func(t *testing.T) {
		got := update.ValidateHeaders(map[string]string{
			"X-Profile-Identity": "custom-identity",
			"X-Profile-Channel":  "custom-channel",
		})
		if got["X-Profile-Identity"] != "custom-identity" || got["X-Profile-Channel"] != "custom-channel" {
			t.Errorf("got %#v, want profile identity/channel headers to pass through", got)
		}
	})

	t.Run("caps total header count deterministically", func(t *testing.T) {
		raw := make(map[string]string, update.MaxCustomHeaders+5)
		for i := 0; i < update.MaxCustomHeaders+5; i++ {
			raw[strings.Repeat("X", i+1)] = "v"
		}

		// Run twice. Map iteration order is random.
		// This catches a regression back to non-deterministic capping.
		first := update.ValidateHeaders(raw)
		second := update.ValidateHeaders(raw)

		if len(first) != update.MaxCustomHeaders {
			t.Fatalf("got %d headers, want %d", len(first), update.MaxCustomHeaders)
		}
		for key := range first {
			if _, ok := second[key]; !ok {
				t.Errorf("header %q survived one run but not another, capping is not deterministic", key)
			}
		}

		// Keys are "X", "XX", "XXX", ... "XXXXX...". Sorted, the shortest
		// keys always win. "X" through the 20-char key must survive.
		if _, ok := first[strings.Repeat("X", update.MaxCustomHeaders)]; !ok {
			t.Errorf("expected the shortest %d keys to survive, got %#v", update.MaxCustomHeaders, first)
		}
	})
}

func TestLoadCustomHeadersFromFile(t *testing.T) {
	t.Run("missing file errors", func(t *testing.T) {
		_, err := update.LoadCustomHeadersFromFile(filepath.Join(t.TempDir(), "does-not-exist.conf"))
		if err == nil {
			t.Fatal("expected an error for a missing file")
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		fn := writeTempConf(t, "not json")
		_, err := update.LoadCustomHeadersFromFile(fn)
		if err == nil {
			t.Fatal("expected an error for invalid JSON")
		}
	})

	t.Run("oversized file errors", func(t *testing.T) {
		big := `{"Headers":{"X-Big":"` + strings.Repeat("a", update.MaxHeadersFileBytes) + `"}}`
		fn := writeTempConf(t, big)
		_, err := update.LoadCustomHeadersFromFile(fn)
		if err == nil {
			t.Fatal("expected an error for a file exceeding the size limit")
		}
	})

	t.Run("valid file returns validated headers", func(t *testing.T) {
		fn := writeTempConf(t, `{"Headers":{"X-Custom-Tenant":"tenant-42","X-Custom-Routing":"us-east-edge"}}`)
		got, err := update.LoadCustomHeadersFromFile(fn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["X-Custom-Tenant"] != "tenant-42" || got["X-Custom-Routing"] != "us-east-edge" {
			t.Errorf("got %#v", got)
		}
	})
}

func TestAddCustomHeaders(t *testing.T) {
	t.Run("fails open gracefully when file missing", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "https://example.com", nil)
		update.AddCustomHeaders(req)
		if len(req.Header) != 0 {
			t.Errorf("expected no headers added when updater.conf is missing, got %#v", req.Header)
		}
	})

	t.Run("attaches headers when file present next to executable", func(t *testing.T) {
		bin, err := os.Executable()
		if err != nil {
			t.Skipf("cannot resolve os.Executable: %v", err)
		}
		confPath := filepath.Join(filepath.Dir(bin), "updater.conf")
		// Prevent overwriting an existing file
		if _, err := os.Stat(confPath); err == nil {
			t.Skip("updater.conf already exists in executable directory")
		}
		content := `{"Headers":{"X-Custom-Tenant":"tenant-42"}}`
		if err := os.WriteFile(confPath, []byte(content), 0600); err != nil {
			t.Skipf("cannot write temporary updater.conf next to executable: %v", err)
		}
		t.Cleanup(func() {
			os.Remove(confPath)
		})

		req, _ := http.NewRequest("GET", "https://example.com", nil)
		update.AddCustomHeaders(req)

		if req.Header.Get("X-Custom-Tenant") != "tenant-42" {
			t.Errorf("got %q, want %q", req.Header.Get("X-Custom-Tenant"), "tenant-42")
		}
	})
}

func writeTempConf(t *testing.T, content string) string {
	t.Helper()
	fn := filepath.Join(t.TempDir(), "updater.conf")
	if err := os.WriteFile(fn, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return fn
}
