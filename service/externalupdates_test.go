package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kardianos/service"
	"github.com/papercutsoftware/silver/service/config"
)

const (
	testPoll    = 10 * time.Millisecond
	testTimeout = 5 * time.Second
)

// runs an externalUpdateWatcher against a temp file
type watcherHarness struct {
	t        *testing.T
	file     string
	out      *syncBuffer
	done     chan struct{}
	finished chan struct{}

	mu        sync.Mutex
	loads     int
	restarts  []string
	loadErr   func(n int) error    // optional: error for the nth (1-based) load
	restartOK func(n int) bool     // optional: result of the nth (1-based) restart; default true
	confHook  func(*config.Config) // optional: adjusts each loaded config (e.g. service paths)
	firstLoad time.Time

	stopOnce sync.Once
}

func newWatcherHarness(t *testing.T) *watcherHarness {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &watcherHarness{
		t:        t,
		file:     filepath.Join(dir, ".version"),
		out:      &syncBuffer{},
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (h *watcherHarness) write(v string) {
	h.t.Helper()
	if err := os.WriteFile(h.file, []byte(v), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *watcherHarness) load() (*config.Config, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.loads++
	if h.loads == 1 {
		h.firstLoad = time.Now()
	}
	if h.loadErr != nil {
		if err := h.loadErr(h.loads); err != nil {
			return nil, err
		}
	}
	data, _ := os.ReadFile(h.file)
	conf := emptyConf()
	conf.EnvironmentVars["VERSION"] = strings.TrimSpace(string(data))
	if h.confHook != nil {
		h.confHook(conf)
	}
	return conf, nil
}

func (h *watcherHarness) restart(conf *config.Config) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.restarts = append(h.restarts, conf.EnvironmentVars["VERSION"])
	if h.restartOK != nil {
		return h.restartOK(len(h.restarts))
	}
	return true
}

// start watches only the component holding h.file, running the given version, before and after every restart
func (h *watcherHarness) start(running string, poll, maxDelay time.Duration) {
	h.t.Helper()
	c := component{dir: filepath.Dir(h.file)}
	h.startWatching(map[component]string{c: running}, func(*config.Config) []component { return []component{c} }, poll, maxDelay)
}

func (h *watcherHarness) startWatching(running map[component]string, components func(*config.Config) []component, poll, maxDelay time.Duration) {
	h.t.Helper()
	w := &externalUpdateWatcher{
		Components:      components,
		RunningVersions: running,
		Poll:            poll,
		MaxRandomDelay:  maxDelay,
		Load:            h.load,
		Restart:         h.restart,
		Done:            h.done,
		Logger:          log.New(h.out, "", 0),
	}
	go func() {
		defer close(h.finished)
		w.Run()
	}()
	h.t.Cleanup(func() {
		h.stop()
		select {
		case <-h.finished:
		case <-time.After(testTimeout):
			h.t.Error("Run did not return after Done was closed")
		}
	})
}

func (h *watcherHarness) stop() {
	h.stopOnce.Do(func() { close(h.done) })
}

func (h *watcherHarness) snapshot() (loads int, restarts []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.loads, append([]string(nil), h.restarts...)
}

func (h *watcherHarness) waitFor(what string, cond func(loads int, restarts []string) bool) {
	h.t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		loads, restarts := h.snapshot()
		if cond(loads, restarts) {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s (loads=%d restarts=%q)\nlog:\n%s", what, loads, restarts, h.out.String())
		}
		time.Sleep(time.Millisecond)
	}
}

// a settled change restarts once; unchanged, empty or superseded values don't; failed loads are retried
func TestExternalUpdateWatcher_Scenarios(t *testing.T) {
	type step struct {
		content string
		pause   time.Duration // sleep after this write
	}
	const slowPoll = 40 * time.Millisecond
	tests := []struct {
		name           string
		poll           time.Duration // 0: testPoll
		runningVersion string
		initial        *string // nil: no file at start
		writes         []step
		loadFailures   int    // fail the first N loads with loadErrText
		loadErrText    string // when set, also assert how often it is logged
		settlePolls    int    // polls to let pass before asserting exact counts
		wantRestarts   []string
		wantLoads      int
		wantErrLogs    int
	}{
		// restarts on change
		{name: "changed version", runningVersion: "1.0", initial: strPtr("1.0"), writes: []step{{content: "1.1"}},
			settlePolls: 10, wantRestarts: []string{"1.1"}, wantLoads: 1},
		{name: "whitespace trimmed", runningVersion: "1.0", initial: strPtr("1.0\n"), writes: []step{{content: "  2.0\r\n"}},
			settlePolls: 10, wantRestarts: []string{"2.0"}, wantLoads: 1},
		{name: "missing file then value", runningVersion: "", initial: nil, writes: []step{{content: "1.0"}},
			settlePolls: 10, wantRestarts: []string{"1.0"}, wantLoads: 1},
		// empty or unchanged shouldn't trigger a restart
		{name: "same as running version", runningVersion: "1.0", initial: strPtr("1.0"), settlePolls: 15},
		{name: "same as running version with whitespace", runningVersion: "1.0", initial: strPtr(" 1.0 \n"), settlePolls: 15},
		{name: "empty file", runningVersion: "1.0", initial: strPtr(""), settlePolls: 15},
		{name: "whitespace only", runningVersion: "1.0", initial: strPtr(" \n\t"), settlePolls: 15},
		{name: "missing file", runningVersion: "1.0", initial: nil, settlePolls: 15},
		{name: "missing file and no running version", runningVersion: "", initial: nil, settlePolls: 15},
		// don't preemptively change the running version to intermediate values
		{name: "replaced by newer value", poll: slowPoll, runningVersion: "1.0", initial: strPtr("1.0"),
			writes:      []step{{content: "1.1", pause: slowPoll / 2}, {content: "1.2"}},
			settlePolls: 5, wantRestarts: []string{"1.2"}, wantLoads: 1},
		{name: "reverted to running version", poll: slowPoll, runningVersion: "1.0", initial: strPtr("1.0"),
			writes:      []step{{content: "1.1", pause: slowPoll / 2}, {content: "1.0"}},
			settlePolls: 6},
		// a failed load is retried every poll, logged once, and never restarts
		{name: "load failure retried", runningVersion: "1.0", initial: strPtr("1.0"), writes: []step{{content: "1.1"}},
			loadFailures: 4, loadErrText: "config is broken xyzzy",
			settlePolls: 5, wantRestarts: []string{"1.1"}, wantLoads: 5, wantErrLogs: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			poll := tc.poll
			if poll == 0 {
				poll = testPoll
			}
			h := newWatcherHarness(t)
			if tc.loadFailures > 0 {
				h.loadErr = func(n int) error {
					if n <= tc.loadFailures {
						return errors.New(tc.loadErrText)
					}
					return nil
				}
			}
			if tc.initial != nil {
				h.write(*tc.initial)
			}
			h.start(tc.runningVersion, poll, 5*time.Millisecond)

			for _, w := range tc.writes {
				h.write(w.content)
				time.Sleep(w.pause)
			}
			if len(tc.wantRestarts) > 0 {
				h.waitFor("restart", func(_ int, r []string) bool { return len(r) > 0 })
			}
			// Several more polls with the file unchanged: a settled value is now the running version.
			time.Sleep(time.Duration(tc.settlePolls) * poll)

			loads, restarts := h.snapshot()
			if fmt.Sprintf("%q", restarts) != fmt.Sprintf("%q", tc.wantRestarts) {
				t.Errorf("restarts = %q, want exactly %q", restarts, tc.wantRestarts)
			}
			if loads != tc.wantLoads {
				t.Errorf("loads = %d, want %d", loads, tc.wantLoads)
			}
			if tc.loadErrText != "" {
				if n := strings.Count(h.out.String(), tc.loadErrText); n != tc.wantErrLogs {
					t.Errorf("error logged %d times, want %d\nlog:\n%s", n, tc.wantErrLogs, h.out.String())
				}
			}
		})
	}
}

// two consecutive results for a given value triggers a restart/settle
func TestExternalUpdateWatcher_WaitsOnePollToSettle(t *testing.T) {
	const poll = 150 * time.Millisecond
	h := newWatcherHarness(t)
	h.write("1.1")
	started := time.Now()
	h.start("1.0", poll, time.Millisecond)
	h.waitFor("load", func(l int, _ []string) bool { return l > 0 })

	h.mu.Lock()
	elapsed := h.firstLoad.Sub(started)
	h.mu.Unlock()
	if elapsed < poll*3/2 {
		t.Errorf("config loaded %v after Run started, want at least 1.5 polls (%v): the new value must settle for one poll", elapsed, poll*3/2)
	}
}

// don't change the running version if restarting fails
func TestExternalUpdateWatcher_RestartFailureKeepsRunningVersion(t *testing.T) {
	h := newWatcherHarness(t)
	h.restartOK = func(int) bool { return false }
	h.write("1.0")
	h.start("1.0", testPoll, time.Millisecond)
	h.write("1.1")

	h.waitFor("Restart retried", func(_ int, r []string) bool { return len(r) >= 2 })
	_, restarts := h.snapshot()
	for i, v := range restarts {
		if v != "1.1" {
			t.Errorf("restarts[%d] = %q, want \"1.1\" (restarts=%q)", i, v, restarts)
		}
	}
}

func TestExternalUpdateWatcher_StopEndsWaits(t *testing.T) {
	tests := []struct {
		name     string
		poll     time.Duration
		maxDelay time.Duration
		settle   bool // write a new version and wait until it is loaded before stopping
	}{
		{name: "during poll wait", poll: time.Hour, maxDelay: time.Millisecond},
		{name: "during random delay", poll: testPoll, maxDelay: time.Hour, settle: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newWatcherHarness(t)
			h.write("1.0")
			h.start("1.0", tc.poll, tc.maxDelay)
			if tc.settle {
				h.write("1.1")
				h.waitFor("load", func(l int, _ []string) bool { return l > 0 })
			}

			h.stop()
			select {
			case <-h.finished:
			case <-time.After(time.Second):
				t.Fatal("Run did not return promptly after Done was closed")
			}
			if _, restarts := h.snapshot(); len(restarts) != 0 {
				t.Errorf("restarts = %q, want none after stop", restarts)
			}
		})
	}
}

// the watcher should retry the config each poll, if it failed to pass validation
func TestExternalUpdateWatcher_UnrunnableConfigRetried(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, []string{"v1/sub.conf"}, nil)
	exe := filepath.Join(root, "v*", "x.exe")

	h := newWatcherHarness(t)
	h.confHook = func(c *config.Config) {
		c.Include = []string{filepath.Join(root, "v*", "sub.conf")}
		c.Services = []config.Service{svcAt(exe)}
	}
	h.write("1.0")
	h.start("1.0", testPoll, time.Millisecond)
	h.write("1.1")

	h.waitFor("repeated loads", func(l int, _ []string) bool { return l >= 4 })
	time.Sleep(3 * testPoll)
	if _, restarts := h.snapshot(); len(restarts) != 0 {
		t.Fatalf("restarts = %q, want none while the config is not runnable", restarts)
	}
	if n := strings.Count(h.out.String(), exe); n != 1 {
		t.Errorf("failing path %q logged %d times, want 1\nlog:\n%s", exe, n, h.out.String())
	}

	makeTree(t, root, []string{"v1/x.exe"}, nil)
	h.waitFor("restart after fix", func(_ int, r []string) bool { return len(r) > 0 })
	time.Sleep(5 * testPoll)
	if _, restarts := h.snapshot(); fmt.Sprintf("%q", restarts) != fmt.Sprintf("%q", []string{"1.1"}) {
		t.Errorf("restarts = %q, want exactly [\"1.1\"]", restarts)
	}
}

// each component's .version file is watched separately; the list follows the config after each restart
func TestExternalUpdateWatcher_ComponentFiles(t *testing.T) {
	const poll = 40 * time.Millisecond
	type phase struct {
		writes       map[string]string // component -> new .version content, written together
		wantRestarts int               // total restarts once this phase has settled
	}
	tests := []struct {
		name            string
		initial         map[string]string // component -> .version content before start
		runningVersions map[string]string // component -> running version; also the initially watched components
		loaded          string            // components used by every config Load returns (comma separated)
		phases          []phase
		wantLog         bool // the watcher must log (once) that there is nothing to watch
	}{
		{name: "change in watched component", initial: map[string]string{"beta": "1.0"}, runningVersions: map[string]string{"beta": "1.0"},
			loaded: "beta", phases: []phase{{writes: map[string]string{"beta": "1.1"}, wantRestarts: 1}}},
		{name: "change in unwatched component", initial: map[string]string{"alpha": "1.0", "beta": "1.0"}, runningVersions: map[string]string{"beta": "1.0"},
			loaded: "beta", phases: []phase{{writes: map[string]string{"alpha": "1.1"}, wantRestarts: 0}}},
		// both new values are on disk before start, so the first poll sees both changes and they settle together
		{name: "two components change in one poll", initial: map[string]string{"alpha": "1.1", "beta": "2.1"},
			runningVersions: map[string]string{"alpha": "1.0", "beta": "1.0"}, loaded: "alpha,beta",
			phases: []phase{
				{wantRestarts: 1},
				// both running versions advanced: no further restarts
				{wantRestarts: 1},
			}},
		{name: "restarted config adds a component", initial: map[string]string{"alpha": "5.0", "beta": "1.0"},
			runningVersions: map[string]string{"beta": "1.0"}, loaded: "alpha,beta",
			phases: []phase{
				// beta's change restarts; alpha is then watched with no running version, so its value restarts once more
				{writes: map[string]string{"beta": "1.1"}, wantRestarts: 2},
				// alpha is now settled: only a later change restarts
				{wantRestarts: 2},
				{writes: map[string]string{"alpha": "5.1"}, wantRestarts: 3},
			}},
		{name: "empty list", initial: map[string]string{"alpha": "1.0"}, runningVersions: map[string]string{}, loaded: "",
			phases: []phase{{writes: map[string]string{"alpha": "1.1", "beta": "1.0"}, wantRestarts: 0}}, wantLog: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newWatcherHarness(t)
			root := filepath.Dir(h.file)
			fileOf := func(component string) string { return filepath.Join(root, component, ".version") }
			write := func(component, v string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(fileOf(component)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fileOf(component), []byte(v), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for c, v := range tc.initial {
				write(c, v)
			}
			running := map[component]string{}
			for c, v := range tc.runningVersions {
				running[component{dir: filepath.Dir(fileOf(c))}] = v
			}
			h.confHook = func(c *config.Config) { c.EnvironmentVars["COMPONENTS"] = tc.loaded }
			components := func(c *config.Config) []component {
				var out []component
				for _, comp := range strings.Split(c.EnvironmentVars["COMPONENTS"], ",") {
					if comp != "" {
						out = append(out, component{dir: filepath.Dir(fileOf(comp))})
					}
				}
				return out
			}
			h.startWatching(running, components, poll, time.Millisecond)

			for i, ph := range tc.phases {
				for c, v := range ph.writes {
					write(c, v)
				}
				if ph.wantRestarts > 0 {
					h.waitFor(fmt.Sprintf("phase %d restarts", i), func(_ int, r []string) bool { return len(r) >= ph.wantRestarts })
				}
				time.Sleep(6 * poll)
				if _, restarts := h.snapshot(); len(restarts) != ph.wantRestarts {
					t.Fatalf("phase %d: %d restarts, want exactly %d\nlog:\n%s", i, len(restarts), ph.wantRestarts, h.out.String())
				}
			}
			if tc.wantLog {
				first := h.out.String()
				if first == "" {
					t.Fatal("nothing logged for an empty list of version files")
				}
				time.Sleep(5 * poll)
				if again := h.out.String(); again != first {
					t.Errorf("log grew after the first message, want it logged once\nbefore:\n%s\nafter:\n%s", first, again)
				}
				if loads, _ := h.snapshot(); loads != 0 {
					t.Errorf("loads = %d, want 0 with nothing to watch", loads)
				}
			}
		})
	}
}

// versions are read before the config that will run is loaded, so an update landing in between is not taken as running
func TestLoadConfAndRunningVersions(t *testing.T) {
	tests := []struct {
		name    string
		errOn   int // fail this (1-based) load; 0: none
		wantErr bool
	}{
		{name: "update during startup"},
		{name: "first load fails", errOn: 1, wantErr: true},
		{name: "second load fails", errOn: 2, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			makeTree(t, dir, []string{"v1/sub.conf"}, nil)
			versionFile := filepath.Join(dir, ".version")
			if err := os.WriteFile(versionFile, []byte("1.0\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			loadErr := errors.New("load failed xyzzy")
			var calls int
			var confs []*config.Config
			load := func() (*config.Config, error) {
				calls++
				if calls == tc.errOn {
					return nil, loadErr
				}
				if calls == 2 {
					if err := os.WriteFile(versionFile, []byte("2.0\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				c := emptyConf()
				c.Include = []string{filepath.Join(dir, "v*", "sub.conf")}
				confs = append(confs, c)
				return c, nil
			}

			conf, versions, err := loadConfAndRunningVersions(load)
			if tc.wantErr {
				if !errors.Is(err, loadErr) {
					t.Fatalf("error = %v, want %v", err, loadErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfAndRunningVersions() error = %v", err)
			}
			if calls != 2 {
				t.Errorf("load called %d times, want 2", calls)
			}
			if len(confs) != 2 || conf != confs[1] {
				t.Errorf("returned config is not the one from the second load")
			}
			want := map[component]string{{dir: dir}: "1.0"}
			if !reflect.DeepEqual(versions, want) {
				t.Errorf("versions = %v, want %v", versions, want)
			}
		})
	}
}

type fakeService struct{ service.Service }

func (fakeService) Logger(chan<- error) (service.Logger, error) {
	return nil, errors.New("no system logger in tests")
}

func TestStart_ChoosesWatcherByServiceLevel(t *testing.T) {
	tests := []struct {
		name         string
		allUsers     *bool // nil: placeholder (asserted false)
		wantExternal bool
	}{
		{name: "default placeholder", allUsers: nil, wantExternal: false},
		{name: "all users", allUsers: boolPtr(true), wantExternal: true},
		{name: "not all users", allUsers: boolPtr(false), wantExternal: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.allUsers == nil && isAllUsersService(emptyConf()) {
				t.Fatal("isAllUsersService returned true, want false until ServiceLevel exists")
			}
			origAll, origReload, origExternal := isAllUsersService, startReloadWatcher, startExternalWatcher
			t.Cleanup(func() {
				isAllUsersService, startReloadWatcher, startExternalWatcher = origAll, origReload, origExternal
			})
			if tc.allUsers != nil {
				v := *tc.allUsers
				isAllUsersService = func(*config.Config) bool { return v }
			}
			var reloadCalls, externalCalls int
			startReloadWatcher = func(*context) { reloadCalls++ }
			startExternalWatcher = func(*context) { externalCalls++ }

			var out syncBuffer
			ctx := newTestContext(emptyConf(), &out)
			o := &osService{ctx: ctx}
			if err := o.Start(fakeService{}); err != nil {
				t.Fatalf("Start returned %v", err)
			}
			t.Cleanup(func() { stopServices(ctx) })

			// Start calls the chosen hook synchronously, so the counts are final once it returns.
			wantReload, wantExternal := 1, 0
			if tc.wantExternal {
				wantReload, wantExternal = 0, 1
			}
			if reloadCalls != wantReload || externalCalls != wantExternal {
				t.Errorf("reload watcher started %d times, external watcher %d times; want %d and %d", reloadCalls, externalCalls, wantReload, wantExternal)
			}
			if tc.wantExternal && !strings.Contains(out.String(), ".version") {
				t.Errorf("startup log does not name the version file:\n%s", out.String())
			}
		})
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
