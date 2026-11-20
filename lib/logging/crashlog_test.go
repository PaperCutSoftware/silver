// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

// Do not use t.Parallel() here; these tests swap the package-level setCrashOutput.

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func stubCrashOutput(t *testing.T) **os.File {
	t.Helper()
	var got *os.File
	orig := setCrashOutput
	setCrashOutput = func(f *os.File, _ debug.CrashOptions) error {
		got = f
		return nil
	}
	t.Cleanup(func() { setCrashOutput = orig })
	return &got
}

func TestSetupCrashOutput_EnablesCrashOutput(t *testing.T) {
	called := stubCrashOutput(t)
	path := filepath.Join(t.TempDir(), "x-silver-crash.log")
	var buf bytes.Buffer

	SetupCrashOutput(path, "", log.New(&buf, "", 0))

	if *called == nil {
		t.Fatal("expected setCrashOutput to be called with a file")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected crash log to be created: %v", err)
	}
	if !strings.Contains(buf.String(), "Crash output enabled: "+path) {
		t.Errorf("expected confirmation in log, got %q", buf.String())
	}
}

func TestSetupCrashOutput_ChownFailureStillEnables(t *testing.T) {
	called := stubCrashOutput(t)
	origChown := changeOwnerOfFileFunc
	changeOwnerOfFileFunc = func(string, string) error { return errors.New("chown failed") }
	t.Cleanup(func() { changeOwnerOfFileFunc = origChown })
	path := filepath.Join(t.TempDir(), "x-silver-crash.log")
	var buf bytes.Buffer

	SetupCrashOutput(path, "someone", log.New(&buf, "", 0))

	if *called == nil {
		t.Fatal("expected setCrashOutput to still be called after chown failure")
	}
	if !strings.Contains(buf.String(), "WARNING: Unable to set owner of crash log") {
		t.Errorf("expected owner warning, got %q", buf.String())
	}
}

func TestSetupCrashOutput_UnwritablePathWarnsAndSkips(t *testing.T) {
	called := stubCrashOutput(t)
	path := filepath.Join(t.TempDir(), "missing-dir", "x-silver-crash.log")
	var buf bytes.Buffer

	SetupCrashOutput(path, "", log.New(&buf, "", 0))

	if *called != nil {
		t.Error("expected setCrashOutput not to be called")
	}
	if !strings.Contains(buf.String(), "WARNING: Unable to open crash log") {
		t.Errorf("expected open warning, got %q", buf.String())
	}
}
