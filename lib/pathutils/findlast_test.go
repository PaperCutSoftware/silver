// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package pathutils

import (
	"strings"
	"testing"
)

func TestFindLastFile(t *testing.T) {

	var globTests = []struct {
		pattern, expected string
	}{
		{"findlast.go", "findlast.go"},
		{"f*ast.go", "findlast.go"},
		{"f*.go", "findlast_test.go"},
	}

	for _, tt := range globTests {
		pattern := tt.pattern
		expected := tt.expected

		match := FindLastFile(pattern)
		if !strings.Contains(match, expected) {
			t.Errorf("Expected '%s', got '%s'", expected, match)
			continue
		}
	}
}

func TestNoMatch(t *testing.T) {

	match := FindLastFile("*/*/*no-match*/file.exe")
	if !strings.Contains(match, "*") {
		t.Errorf("Did not expect a match on unknown file")
	}
}
