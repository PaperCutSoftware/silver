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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"os"
	"regexp"
	"sort"
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
//
// Host, Content-Length, Transfer-Encoding, and Connection are also denied.
// Go's net/http manages these itself. It ignores whatever is set on
// req.Header for them. Allowing them through would silently do nothing.
// That's a confusing trap for an operator configuring updater.conf.
var systemHeaderKeys = map[string]struct{}{
	textproto.CanonicalMIMEHeaderKey(headerUserAgentKey):       {},
	textproto.CanonicalMIMEHeaderKey(headerProfileTimezoneKey): {},
	textproto.CanonicalMIMEHeaderKey(headerOSTypeKey):          {},
	textproto.CanonicalMIMEHeaderKey(headerOSVersionKey):       {},
	textproto.CanonicalMIMEHeaderKey(headerOSArchKey):          {},
	textproto.CanonicalMIMEHeaderKey(headerBinaryArchKey):      {},
	textproto.CanonicalMIMEHeaderKey("Host"):                   {},
	textproto.CanonicalMIMEHeaderKey("Content-Length"):         {},
	textproto.CanonicalMIMEHeaderKey("Transfer-Encoding"):      {},
	textproto.CanonicalMIMEHeaderKey("Connection"):             {},
}

func isSystemHeader(canonical string) bool {
	_, ok := systemHeaderKeys[canonical]
	return ok
}

type headersFile struct {
	Headers map[string]string `json:"Headers"`
}

// AddCustomHeaders attaches operator-defined headers from updater.conf
// to req. It fails open on any error.
//
// Callers must invoke AddCustomHeaders after legacy updater-profile.conf
// headers are set. Call it before User-Agent and OS headers are set.
func AddCustomHeaders(req *http.Request) {
	headers, err := loadCustomHeaders()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("%s not found. Proceeding without custom headers.\n", headersFileName)
		} else {
			fmt.Printf("Couldn't load custom headers: %v.\n", err)
		}
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

	// Read one byte past the limit. An oversized file becomes an error.
	// It is not silently truncated.
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
	// Sort keys first. Map iteration order is random.
	// Without sorting, which headers get dropped past maxCustomHeaders
	// would differ from run to run.
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// "X-Foo" and "x-foo" canonicalize to the same header.
	// Keep only the first one seen. This keeps the winner deterministic.
	// Otherwise random map iteration order would decide it later.
	seenCanonical := make(map[string]bool, len(raw))

	valid := make(map[string]string, len(raw))
	for i, key := range keys {
		if len(valid) >= maxCustomHeaders {
			fmt.Printf("%s defines more than %d valid headers. Ignoring the remaining %d.\n",
				headersFileName, maxCustomHeaders, len(keys)-i)
			break
		}

		value := raw[key]
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		canonical := textproto.CanonicalMIMEHeaderKey(key)

		switch {
		case !isValidHeaderFieldName(key):
			fmt.Printf("Ignoring custom header %q. Invalid header field name.\n", key)
		case len(value) > maxHeaderValueBytes:
			fmt.Printf("Ignoring custom header %q. Value exceeds %d bytes.\n", key, maxHeaderValueBytes)
		case !isValidHeaderFieldValue(value):
			fmt.Printf("Ignoring custom header %q. Invalid header field value.\n", key)
		case isSystemHeader(canonical):
			fmt.Printf("Ignoring custom header %q. Reserved for internal use.\n", key)
		case seenCanonical[canonical]:
			fmt.Printf("Ignoring custom header %q. Duplicate of an already-configured header.\n", key)
		default:
			valid[key] = value
			seenCanonical[canonical] = true
		}
	}
	return valid
}

func getHeadersFileName() (string, error) {
	return fileNextToExecutable(headersFileName)
}

// reHeaderFieldName matches a non-empty RFC 9110 token.
// That's the valid form for an HTTP field-name.
// See https://www.rfc-editor.org/rfc/rfc9110.html#section-5.6.2
var reHeaderFieldName = regexp.MustCompile("^[\\w!#$%&'*+\\-.^`|~]+$")

func isValidHeaderFieldName(s string) bool {
	return reHeaderFieldName.MatchString(s)
}

// isValidHeaderFieldValue reports whether s is a valid RFC 9110 field-value.
// Valid means visible ASCII or obs-text bytes, plus internal tabs.
// Control characters are rejected, notably CR/LF.
// Those could otherwise inject extra header lines.
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
