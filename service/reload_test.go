package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/service/config"
)

// Fixed, distinct modification times. Using os.Chtimes avoids sleeping and
// relying on filesystem timestamp resolution.
var (
	mtime1 = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	mtime2 = time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	mtime3 = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	mtime0 = time.Date(2023, 6, 1, 9, 0, 0, 0, time.UTC) // older than mtime1
)

func TestReloadSignal_ActiveConsumesFile(t *testing.T) {
	_, path := reloadPath(t)
	r := newReloadSignal(path, false)

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

// --- helpers -----------------------------------------------------------------

// reloadPath returns a fresh temp directory and the reload-file path inside it.
func reloadPath(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	return dir, filepath.Join(dir, ".reload")
}

// writeReloadFile creates (or truncates) path and sets its modification time.
func writeReloadFile(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	setMtime(t, path, mtime)
}

func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func removeReloadFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatalf("unexpected stat error for %s: %v", path, err)
	return false
}

type dirEntrySnapshot struct {
	name  string
	size  int64
	mtime time.Time
	mode  os.FileMode
}

type dirSnapshot struct {
	dirMtime time.Time
	entries  []dirEntrySnapshot
}

// snapshotDir records everything a write, create or remove in dir would change.
func snapshotDir(t *testing.T, dir string) dirSnapshot {
	t.Helper()
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap := dirSnapshot{dirMtime: dirInfo.ModTime()}
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			t.Fatal(err)
		}
		snap.entries = append(snap.entries, dirEntrySnapshot{
			name:  de.Name(),
			size:  info.Size(),
			mtime: info.ModTime(),
			mode:  info.Mode(),
		})
	}
	sort.Slice(snap.entries, func(i, j int) bool { return snap.entries[i].name < snap.entries[j].name })
	return snap
}

func assertSnapshotsEqual(t *testing.T, before, after dirSnapshot) {
	t.Helper()
	if !before.dirMtime.Equal(after.dirMtime) {
		t.Fatalf("directory modification time changed during Poll(): %v -> %v", before.dirMtime, after.dirMtime)
	}
	if len(before.entries) != len(after.entries) {
		t.Fatalf("directory entries changed during Poll(): %+v -> %+v", before.entries, after.entries)
	}
	for i := range before.entries {
		b, a := before.entries[i], after.entries[i]
		if b.name != a.name || b.size != a.size || !b.mtime.Equal(a.mtime) || b.mode != a.mode {
			t.Fatalf("directory entry changed during Poll(): %+v -> %+v", b, a)
		}
	}
}

// passivePoller polls a passive reloadSignal and asserts that each poll
// returns the expected value and leaves the directory untouched.
type passivePoller struct {
	t    *testing.T
	r    *reloadSignal
	dir  string
	path string
}

func newPassivePoller(t *testing.T) (*passivePoller, string) {
	t.Helper()
	dir, path := reloadPath(t)
	return &passivePoller{t: t, dir: dir, path: path}, path
}

func (p *passivePoller) start() {
	p.t.Helper()
	existed := fileExists(p.t, p.path)
	before := snapshotDir(p.t, p.dir)
	p.r = newReloadSignal(p.path, true)
	if p.r == nil {
		p.t.Fatal("newReloadSignal returned nil")
	}
	assertSnapshotsEqual(p.t, before, snapshotDir(p.t, p.dir))
	if got := fileExists(p.t, p.path); got != existed {
		p.t.Fatalf("constructor changed file existence: %v -> %v", existed, got)
	}
}

func (p *passivePoller) poll(want bool) {
	p.t.Helper()
	existed := fileExists(p.t, p.path)
	before := snapshotDir(p.t, p.dir)
	if got := p.r.Poll(); got != want {
		p.t.Fatalf("Poll() = %v, want %v", got, want)
	}
	if got := fileExists(p.t, p.path); got != existed {
		p.t.Fatalf("Poll() changed file existence in passive mode: %v -> %v", existed, got)
	}
	assertSnapshotsEqual(p.t, before, snapshotDir(p.t, p.dir))
}

func skipIfPermissionsUnenforced(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions")
	}
}

// lockDir chmods dir to mode and restores 0755 in Cleanup (so TempDir removal works).
func lockDir(t *testing.T, dir string, mode os.FileMode) (unlock func()) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	unlocked := false
	unlock = func() {
		if unlocked {
			return
		}
		unlocked = true
		if err := os.Chmod(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(unlock)
	return unlock
}

// --- passive mode: no change ----------------------------------------------

func TestReloadSignal_PassiveNoChangeStaysFalse(t *testing.T) {
	cases := []struct {
		name     string
		withFile bool
	}{
		{"absent", false},
		{"present", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, path := newPassivePoller(t)
			if tc.withFile {
				writeReloadFile(t, path, mtime1)
			}
			p.start() // also asserts the constructor left the directory untouched

			for i := 0; i < 3; i++ {
				p.poll(false)
			}
			if got := fileExists(t, path); got != tc.withFile {
				if tc.withFile {
					t.Fatal("passive Poll() removed the reload file")
				}
				t.Fatal("passive Poll() created the reload file")
			}
		})
	}
}

// --- passive mode: started without the file ---------------------------------

func TestReloadSignal_PassiveStartedWithoutFile_AppearanceFiresOnceAndKeepsFile(t *testing.T) {
	p, path := newPassivePoller(t)
	p.start()

	p.poll(false)
	writeReloadFile(t, path, mtime1)
	p.poll(true)
	if !fileExists(t, path) {
		t.Fatal("passive Poll() removed the reload file")
	}
	p.poll(false)
	p.poll(false)

	setMtime(t, path, mtime2)
	p.poll(true)
	p.poll(false)
}

// --- passive mode: started with the file ------------------------------------

func TestReloadSignal_PassiveStartedWithFile_MtimeChangeFiresOnce(t *testing.T) {
	cases := []struct {
		name   string
		change time.Time
	}{
		{"newer", mtime2},
		{"older", mtime0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, path := newPassivePoller(t)
			writeReloadFile(t, path, mtime1)
			p.start()

			p.poll(false)
			setMtime(t, path, tc.change)
			p.poll(true)
			p.poll(false)
			p.poll(false)
		})
	}
}

func TestReloadSignal_PassiveStartedWithFile_EachChangeFiresOnce(t *testing.T) {
	// Each element of steps is a batch of mtimes applied between two polls;
	// every batch must yield exactly one true followed by false.
	cases := []struct {
		name  string
		steps [][]time.Time
	}{
		// Back to an earlier value is still a change from the current baseline.
		{"one change per poll", [][]time.Time{{mtime2}, {mtime3}, {mtime1}}},
		// The baseline moves to what was observed, so two changes yield a single true.
		{"two changes between polls", [][]time.Time{{mtime2, mtime3}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, path := newPassivePoller(t)
			writeReloadFile(t, path, mtime1)
			p.start()

			for _, batch := range tc.steps {
				for _, mt := range batch {
					setMtime(t, path, mt)
				}
				p.poll(true)
				p.poll(false)
			}
		})
	}
}

// --- passive mode: deletion and reappearance --------------------------------

func TestReloadSignal_PassiveDeletedThenReappears(t *testing.T) {
	cases := []struct {
		name          string
		startWithFile bool // false: file is written after start() and the baseline is set by a poll
		originalAt    time.Time
		reappearAt    time.Time
	}{
		{"same mtime as original", true, mtime1, mtime1},
		{"newer mtime", true, mtime1, mtime2},
		{"older mtime", true, mtime1, mtime0},
		{"appeared after start, same mtime", false, mtime1, mtime1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, path := newPassivePoller(t)
			if tc.startWithFile {
				writeReloadFile(t, path, tc.originalAt)
				p.start()
				p.poll(false)
			} else {
				p.start()
				writeReloadFile(t, path, tc.originalAt)
				p.poll(true)
			}

			removeReloadFile(t, path)
			p.poll(false)
			p.poll(false)
			if fileExists(t, path) {
				t.Fatal("passive Poll() recreated the deleted reload file")
			}

			writeReloadFile(t, path, tc.reappearAt)
			p.poll(true)
			if !fileExists(t, path) {
				t.Fatal("passive Poll() removed the reappeared reload file")
			}
			p.poll(false)
		})
	}
}

// --- stat errors -------------------------------------------------------------

// pollFalseWhileUnreadable makes dir unsearchable, asserts Poll() reports false
// for the resulting stat error, then restores the directory.
func pollFalseWhileUnreadable(t *testing.T, r *reloadSignal, dir string) {
	t.Helper()
	unlock := lockDir(t, dir, 0)
	if got := r.Poll(); got {
		t.Fatal("Poll() = true on stat error, want false")
	}
	unlock()
}

func TestReloadSignal_PassiveStatErrorReturnsFalseAndKeepsBaseline(t *testing.T) {
	skipIfPermissionsUnenforced(t)

	t.Run("started with file, change pending", func(t *testing.T) {
		dir, path := reloadPath(t)
		writeReloadFile(t, path, mtime1)
		r := newReloadSignal(path, true)

		setMtime(t, path, mtime2)
		pollFalseWhileUnreadable(t, r, dir)

		if !fileExists(t, path) {
			t.Fatal("reload file removed")
		}
		// Baseline was not moved by the failed poll, so the pending change is still seen.
		if got := r.Poll(); !got {
			t.Fatal("Poll() = false after recovering from stat error, want true (baseline should be unchanged)")
		}
		if got := r.Poll(); got {
			t.Fatal("Poll() = true on second poll after recovery, want false")
		}
	})

	t.Run("started with file, no change", func(t *testing.T) {
		dir, path := reloadPath(t)
		writeReloadFile(t, path, mtime1)
		r := newReloadSignal(path, true)

		pollFalseWhileUnreadable(t, r, dir)

		// Baseline must not have been moved to "absent", or this would look like a reappearance.
		if got := r.Poll(); got {
			t.Fatal("Poll() = true after recovering from stat error with unchanged file, want false")
		}
	})

	t.Run("started without file, file appears", func(t *testing.T) {
		dir, path := reloadPath(t)
		r := newReloadSignal(path, true)

		writeReloadFile(t, path, mtime1)
		pollFalseWhileUnreadable(t, r, dir)

		if got := r.Poll(); !got {
			t.Fatal("Poll() = false after recovering from stat error, want true (baseline should be unchanged)")
		}
		if !fileExists(t, path) {
			t.Fatal("passive Poll() removed the reload file")
		}
	})
}

func TestReloadSignal_ActiveStatErrorReturnsFalse(t *testing.T) {
	skipIfPermissionsUnenforced(t)

	dir, path := reloadPath(t)
	r := newReloadSignal(path, false)

	writeReloadFile(t, path, mtime1)
	pollFalseWhileUnreadable(t, r, dir)

	if !fileExists(t, path) {
		t.Fatal("reload file vanished during failed poll")
	}
	if got := r.Poll(); !got {
		t.Fatal("Poll() = false after recovery with file present, want true")
	}
	if fileExists(t, path) {
		t.Fatal("active Poll() did not remove the reload file")
	}
}

func TestReloadSignal_ActiveRemoveFailureReturnsFalse(t *testing.T) {
	skipIfPermissionsUnenforced(t)

	dir, path := reloadPath(t)
	r := newReloadSignal(path, false)

	writeReloadFile(t, path, mtime1)
	// Readable and searchable (stat succeeds) but not writable (remove fails).
	unlock := lockDir(t, dir, 0555)
	if got := r.Poll(); got {
		t.Fatal("Poll() = true when removal failed, want false")
	}
	if !fileExists(t, path) {
		t.Fatal("reload file unexpectedly removed from read-only directory")
	}
	unlock()

	if got := r.Poll(); !got {
		t.Fatal("Poll() = false once removal can succeed, want true")
	}
	if fileExists(t, path) {
		t.Fatal("active Poll() did not remove the reload file")
	}
}

func TestReloadSignal_ConstructorAndPollDoNotPanicOnStatError(t *testing.T) {
	skipIfPermissionsUnenforced(t)

	for _, passive := range []bool{true, false} {
		passive := passive
		t.Run(map[bool]string{true: "passive", false: "active"}[passive], func(t *testing.T) {
			dir, path := reloadPath(t)
			writeReloadFile(t, path, mtime1)
			lockDir(t, dir, 0)

			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("panic: %v", rec)
				}
			}()
			r := newReloadSignal(path, passive)
			if r == nil {
				t.Fatal("newReloadSignal returned nil")
			}
			if got := r.Poll(); got {
				t.Fatal("Poll() = true on stat error, want false")
			}
		})
	}
}

// --- service wiring -----------------------------------------------------------

func TestIsAllUsersService_FalseForEveryConfig(t *testing.T) {
	cases := []struct {
		name string
		conf func(t *testing.T) *config.Config
	}{
		{"zero value", func(t *testing.T) *config.Config { return &config.Config{} }},
		{"logging-defaultdateformat.conf", func(t *testing.T) *config.Config {
			return loadTestConfig(t, "logging-defaultdateformat.conf")
		}},
		{"logging-with-microseconds.conf", func(t *testing.T) *config.Config {
			return loadTestConfig(t, "logging-with-microseconds.conf")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAllUsersService(tc.conf(t)); got {
				t.Fatalf("isAllUsersService() = true, want false")
			}
		})
	}
}
