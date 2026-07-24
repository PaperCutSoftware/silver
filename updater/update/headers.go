// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	headersFileName     = "updater.conf"
	maxHeadersFileBytes = 64 * 1024
	maxHeaderValueBytes = 4096
	maxCustomHeaders    = 20
)

// systemHeaderKeys are headers describing this binary or host itself.
// A custom header sharing one of these names is dropped.
// This provides defense in depth alongside the load-order guarantee in Check().
//
// Legacy profile identity and channel headers are deliberately omitted.
// The new updater.conf is allowed to override updater-profile.conf on collision.
var systemHeaderKeys = map[string]bool{
	textproto.CanonicalMIMEHeaderKey("User-Agent"):             true,
	textproto.CanonicalMIMEHeaderKey(headerProfileTimezoneKey): true,
	textproto.CanonicalMIMEHeaderKey(headerOSTypeKey):          true,
	textproto.CanonicalMIMEHeaderKey(headerOSVersionKey):       true,
	textproto.CanonicalMIMEHeaderKey(headerOSArchKey):          true,
	textproto.CanonicalMIMEHeaderKey(headerBinaryArchKey):      true,
}

type headersFile struct {
	Headers map[string]string `json:"Headers"`
}

// AddCustomHeaders attaches operator-defined headers from updater.conf
// to req. It fails open if locating, reading, parsing, or validating fails.
//
// Callers must invoke AddCustomHeaders after legacy updater-profile.conf
// headers are set, but before User-Agent and OS headers.
func AddCustomHeaders(req *http.Request) {
	headers, err := loadCustomHeaders()
	if err != nil {
		fmt.Printf("Couldn't load custom headers: %v.\n", err)
		return
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func loadCustomHeaders() (map[string]string, error) {
	fn, err := getHeadersFileName()
	if err != nil {
		return nil, err
	}
	return loadCustomHeadersFromFile(fn)
}

func loadCustomHeadersFromFile(fn string) (map[string]string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read one byte past the limit so an oversized file is reported as an
	// error rather than silently truncated.
	data, err := io.ReadAll(io.LimitReader(f, maxHeadersFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxHeadersFileBytes {
		return nil, fmt.Errorf("%s exceeds the %d byte limit", headersFileName, maxHeadersFileBytes)
	}

	var conf headersFile
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", headersFileName, err)
	}

	return validateHeaders(conf.Headers), nil
}

func validateHeaders(raw map[string]string) map[string]string {
	if len(raw) > maxCustomHeaders {
		fmt.Printf("%s defines %d headers; only %d are allowed, the rest will be dropped.\n",
			headersFileName, len(raw), maxCustomHeaders)
	}

	valid := make(map[string]string, len(raw))
	for key, value := range raw {
		if len(valid) >= maxCustomHeaders {
			break
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch {
		case !isValidHeaderFieldName(key):
			fmt.Printf("Ignoring custom header %q: invalid header field name.\n", key)
		case len(value) > maxHeaderValueBytes:
			fmt.Printf("Ignoring custom header %q: value exceeds %d bytes.\n", key, maxHeaderValueBytes)
		case !isValidHeaderFieldValue(value):
			fmt.Printf("Ignoring custom header %q: invalid header field value.\n", key)
		case systemHeaderKeys[textproto.CanonicalMIMEHeaderKey(key)]:
			fmt.Printf("Ignoring custom header %q: reserved for internal use.\n", key)
		default:
			valid[key] = value
		}
	}
	return valid
}

func getHeadersFileName() (string, error) {
	// File containing custom headers should exist alongside the updater binary.
	updaterBin, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(updaterBin), headersFileName), nil
}

// headerFieldNameRE matches a non-empty RFC 9110 token, the valid form for
// an HTTP field-name: https://www.rfc-editor.org/rfc/rfc9110.html#section-5.6.2
var headerFieldNameRE = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+\-.^_` + "`" + `|~]+$`)

func isValidHeaderFieldName(s string) bool {
	return headerFieldNameRE.MatchString(s)
}

// isValidHeaderFieldValue reports whether s is a valid RFC 9110 field-value:
// visible ASCII/obs-text bytes, plus internal tabs, but no control
// characters (notably CR/LF) that could be used to inject header lines.
func isValidHeaderFieldValue(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\t' {
			continue
		}
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}
