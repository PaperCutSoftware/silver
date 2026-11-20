package main

import (
	"path/filepath"
	"testing"
)

func TestCrashLogPath(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("logs", "app.log"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		logFile string
		want    string
	}{
		{"app.log", "app-silver-crash.log"},
		{filepath.Join("logs", "app.log"), filepath.Join("logs", "app-silver-crash.log")},
		{abs, filepath.Join(filepath.Dir(abs), "app-silver-crash.log")},
		{"app", "app-silver-crash.log"},
		{"app.txt", "app-silver-crash.log"},
	}
	for _, tt := range tests {
		if got := crashLogPath(tt.logFile); got != tt.want {
			t.Errorf("crashLogPath(%q) = %q, want %q", tt.logFile, got, tt.want)
		}
	}
}
