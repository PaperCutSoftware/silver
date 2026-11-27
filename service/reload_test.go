package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReloadSignal_ActiveConsumesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".reload")
	r := newReloadSignal(path)

	touch := func() {
		t.Helper()
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	assertPoll := func(want bool) {
		t.Helper()
		if got := r.Poll(); got != want {
			t.Fatalf("Poll() = %v, want %v", got, want)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("reload file still exists after Poll() (stat err: %v)", err)
		}
	}

	assertPoll(false) // absent
	touch()
	assertPoll(true) // present: consumed
	assertPoll(false)
	touch()
	assertPoll(true) // recreated: consumed again
}
