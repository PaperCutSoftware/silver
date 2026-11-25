// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/papercutsoftware/silver/lib/pathutils"
	"github.com/papercutsoftware/silver/service/config"
)

const (
	globChars      = "*?["
	pathSeparators = `/\`
)

// checkConfRunnable returns an error unless every path a restart would start
// exists and, for a path with a wildcard, sits in the correct component's
// current version folder.
func checkConfRunnable(conf *config.Config) error {
	current, err := currentVersionFolders(conf)
	if err != nil {
		return err
	}
	for _, p := range runnablePaths(conf) {
		resolved, err := resolveExisting(p)
		if err != nil {
			return err
		}
		vp, ok := parseVersioned(p)
		if !ok {
			continue
		}
		if got, want := vp.versionOf(resolved), current[vp.component]; want != "" && got != want {
			return fmt.Errorf("%s: resolved to folder '%s', but the component's current version folder is '%s'", p, got, want)
		}
	}
	return nil
}

// runnablePaths lists the paths a restart would start: services, scheduled
// tasks and non-async startup tasks.
func runnablePaths(conf *config.Config) []string {
	var paths []string
	for _, s := range conf.Services {
		paths = append(paths, s.Path)
	}
	for _, t := range conf.ScheduledTasks {
		paths = append(paths, t.Path)
	}
	for _, t := range conf.StartupTasks {
		if !t.Async {
			paths = append(paths, t.Path)
		}
	}
	return paths
}

// isBareName reports whether p is a program name to look up on PATH.
func isBareName(p string) bool {
	return !strings.ContainsAny(p, pathSeparators)
}

// resolveExisting returns the file p resolves to, or an error if there is none.
func resolveExisting(p string) (string, error) {
	if isBareName(p) {
		// deliberately exec.LookPath, as this is what the runtime exec.Command
		// will call as a lookup
		// (lib/procmngt/procmngt.go:131)
		resolved, err := exec.LookPath(p)
		if err != nil {
			return "", fmt.Errorf("%s: not found: %v", p, err)
		}
		return resolved, nil
	}
	resolved := pathutils.FindLastFile(p)
	if fi, err := os.Stat(resolved); err != nil || !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s: missing", p)
	}
	return resolved, nil
}

// currentVersionFolders maps each component (as an absolute path) to its
// current version folder. Config include globs decide it first; a component
// that runnable paths use but no include covers falls back to its newest
// folder, or "" if it has none.
//
// if includes of the same component resolve to different folders, returns
// an error
// e.g.
// Include: ["component1/v*/component1.conf", "component2/v*/component2.conf"]
// -> {"<root>/component1": "v124", "<root>/component2": "v7"}
func currentVersionFolders(conf *config.Config) (map[string]string, error) {
	current := map[string]string{}
	for _, inc := range conf.Include {
		vp, ok := parseVersioned(inc)
		if !ok {
			continue
		}
		resolved := pathutils.FindLastFile(inc)
		if resolved == inc {
			continue // no match
		}
		folder := vp.versionOf(resolved)
		if prev, ok := current[vp.component]; ok && prev != folder {
			return nil, fmt.Errorf("%s: resolved to folder '%s', but the component's earlier includes resolved to '%s'", inc, folder, prev)
		}
		current[vp.component] = folder
	}
	for _, p := range runnablePaths(conf) {
		vp, ok := parseVersioned(p)
		if !ok {
			continue
		}
		if _, ok := current[vp.component]; !ok {
			current[vp.component] = vp.newestVersion()
		}
	}
	return current, nil
}

// versionedPath is a glob path split at its first wildcard path segment:
// "beta/v*/bin/x.exe" -> component "<abs>/beta", versionGlob "v*"
type versionedPath struct {
	component   string // absolute folder holding the version folders
	versionGlob string // e.g. "v*"
}

// parseVersioned splits p at its first wildcard path segment. ok is false when
// p has no wildcard, or is a bare name (which isn't in a component).
func parseVersioned(p string) (v versionedPath, ok bool) {
	prefix, glob, ok := splitAtWildcard(p)
	if !ok || isBareName(p) {
		return versionedPath{}, false
	}
	return versionedPath{component: absPath(prefix), versionGlob: glob}, true
}

// versionOf returns the version folder the resolved path sits in.
func (v versionedPath) versionOf(resolved string) string {
	rel, err := filepath.Rel(v.component, absPath(resolved))
	if err != nil {
		return ""
	}
	return strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
}

// newestVersion returns the newest (by name) folder matching the component's
// version glob, or "" if none.
func (v versionedPath) newestVersion() string {
	matches, _ := filepath.Glob(filepath.Join(v.component, v.versionGlob))
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && fi.IsDir() {
			return filepath.Base(m)
		}
	}
	return ""
}

// componentDir returns the component a path belongs to: the folder above its first
// path segment containing a glob character. ok is false when the path has no wildcard.
func componentDir(p string) (dir string, ok bool) {
	prefix, _, ok := splitAtWildcard(p)
	switch {
	case !ok:
		return "", false
	case prefix == "":
		return ".", true
	case len(prefix) == 1:
		return prefix, true // the root folder
	}
	return prefix[:len(prefix)-1], true
}

// splitAtWildcard finds the first path segment of p containing a glob
// character and returns that segment and everything before it, including the
// trailing separator. ok is false when p has no wildcard.
func splitAtWildcard(p string) (prefix, segment string, ok bool) {
	i := strings.IndexAny(p, globChars)
	if i < 0 {
		return "", "", false
	}
	start := strings.LastIndexAny(p[:i], pathSeparators) + 1
	end := len(p)
	if n := strings.IndexAny(p[i:], pathSeparators); n >= 0 {
		end = i + n
	}
	return p[:start], p[start:end], true
}

// absPath returns p as an absolute, cleaned path with symlinks resolved, so a
// relative path and a ${ServiceRoot} path to the same folder compare equal
// (the working directory is the physical path, the service root may not be).
func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}
