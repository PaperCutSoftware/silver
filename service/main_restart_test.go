package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/service/config"
)

func newTestContext(conf *config.Config, out io.Writer) *context {
	logger := log.New(out, "", 0)
	return &context{
		conf:        conf,
		logger:      logger,
		errorLogger: logger,
		done:        make(chan struct{}),
	}
}

func emptyConf() *config.Config {
	return &config.Config{EnvironmentVars: map[string]string{}}
}

// Concurrent restarts and stops must not panic, and once stopped, nothing restarts.
func TestRestart_StopIsFinal(t *testing.T) {
	ctx := newTestContext(emptyConf(), io.Discard)
	doStart(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); restart(ctx, emptyConf()) }()
		go func() { defer wg.Done(); stopServices(ctx) }()
	}
	wg.Wait()

	if !isClosed(ctx.done) || ctx.cronManager != nil {
		t.Fatal("services still running after stop")
	}
	newConf := emptyConf()
	if stopServices(ctx) || restart(ctx, newConf) || ctx.conf == newConf || ctx.cronManager != nil {
		t.Error("stop or restart acted after the service had stopped")
	}
}

func TestRestart_RealChildAndEnvironment(t *testing.T) {
	const changed, added, removed, kept = "SILVER_TEST_CHANGED", "SILVER_TEST_ADDED", "SILVER_TEST_REMOVED", "SILVER_TEST_KEPT"
	for _, k := range []string{changed, added, removed, kept} {
		t.Setenv(k, "") // restores the original value after the test
		_ = os.Unsetenv(k)
	}
	oldConf := emptyConf()
	oldConf.EnvironmentVars = map[string]string{changed: "old", removed: "gone", kept: "same"}
	setupEnvironment(oldConf)

	var out syncBuffer
	ctx := newTestContext(oldConf, &out)
	doStart(ctx)

	newConf := emptyConf()
	newConf.EnvironmentVars = map[string]string{changed: "new", added: "here", kept: "same"}
	svc := config.Service{GracefulShutdownTimeoutSecs: 5}
	svc.Path = buildHelloForever(t)
	newConf.Services = []config.Service{svc}
	if !restart(ctx, newConf) {
		t.Fatal("restart returned false")
	}

	for k, v := range newConf.EnvironmentVars {
		if got, ok := os.LookupEnv(k); !ok || got != v {
			t.Errorf("%s = %q (set %v), want %q", k, got, ok, v)
		}
	}
	if v, ok := os.LookupEnv(removed); ok {
		t.Errorf("%s still set to %q", removed, v)
	}

	// helloforever prints only after installing its signal handler; stopping earlier would kill it.
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(out.String(), "Hello World") {
		if time.Now().After(deadline) {
			t.Fatalf("child never started:\n%s", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	start := time.Now()
	if !stopServices(ctx) {
		t.Fatal("stopServices returned false")
	}
	if elapsed := time.Since(start); elapsed > 6*time.Second {
		t.Errorf("stop took %v, want within the graceful shutdown timeout", elapsed)
	}
	for _, want := range []string{"Shutting down...", "Service stopped with exit code"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("log missing %q:\n%s", want, out.String())
		}
	}
}

func buildHelloForever(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "svcutil", "testexes", "helloforever.go")
	exe := filepath.Join(t.TempDir(), "helloforever.exe")
	if o, err := exec.Command("go", "build", "-o", exe, src).CombinedOutput(); err != nil {
		t.Fatalf("building helper: %v\n%s", err, o)
	}
	return exe
}

type syncBuffer struct {
	sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.Lock()
	defer b.Unlock()
	return b.buf.String()
}
