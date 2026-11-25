// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/papercutsoftware/silver/service/config"
)

const (
	versionFileName = ".version"
	globChars       = "*?["
	pathSeparators  = `/\`
)

// component is a folder of versioned v* folders plus a .version file, updated
// independently by one updater task: the service root, or a subfolder such as beta.
type component struct{ dir string } // absolute, cleaned, symlink-resolved

// componentOf splits p at its first wildcard path segment:
// "beta/v*/bin/x.exe" -> component "<abs>/beta", versionGlob "v*"
// ok is false when p has no wildcard, or is a bare name (which isn't in a component).
func componentOf(p string) (c component, versionGlob string, ok bool) {
	prefix, glob, ok := splitAtWildcard(p)
	if !ok || isBareName(p) {
		return component{}, "", false
	}
	return component{dir: absPath(prefix)}, glob, true
}

// componentsOf returns the components that the config's includes, services,
// startup tasks and scheduled tasks use, sorted by folder.
func componentsOf(conf *config.Config) []component {
	paths := append([]string(nil), conf.Include...)
	for _, s := range conf.Services {
		paths = append(paths, s.Path)
	}
	for _, t := range conf.StartupTasks {
		paths = append(paths, t.Path)
	}
	for _, t := range conf.ScheduledTasks {
		paths = append(paths, t.Path)
	}
	seen := map[component]bool{}
	var cs []component
	for _, p := range paths {
		c, _, ok := componentOf(p)
		if !ok || seen[c] {
			continue
		}
		seen[c] = true
		cs = append(cs, c)
	}
	sortByDir(cs)
	return cs
}

// sortedComponents returns the components of m, sorted by folder.
func sortedComponents(m map[component]string) []component {
	cs := make([]component, 0, len(m))
	for c := range m {
		cs = append(cs, c)
	}
	sortByDir(cs)
	return cs
}

func sortByDir(cs []component) {
	sort.Slice(cs, func(i, j int) bool { return cs[i].dir < cs[j].dir })
}

func (c component) versionFile() string {
	return filepath.Join(c.dir, versionFileName)
}

// readVersion returns the version in the component's version file, "" when missing
func (c component) readVersion() string {
	data, err := os.ReadFile(c.versionFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// versionOf returns the version folder the resolved path sits in.
func (c component) versionOf(resolved string) string {
	rel, err := filepath.Rel(c.dir, absPath(resolved))
	if err != nil {
		return ""
	}
	return strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
}

// newestFolder returns the newest (by name) folder matching the component's
// version glob, or "" if none.
func (c component) newestFolder(versionGlob string) string {
	matches, _ := filepath.Glob(filepath.Join(c.dir, versionGlob))
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && fi.IsDir() {
			return filepath.Base(m)
		}
	}
	return ""
}

func readRunningVersions(cs []component) map[component]string {
	versions := map[component]string{}
	for _, c := range cs {
		versions[c] = c.readVersion()
	}
	return versions
}

// versionFiles returns the version files of the components of versions, sorted by folder.
func versionFiles(versions map[component]string) []string {
	var files []string
	for _, c := range sortedComponents(versions) {
		files = append(files, c.versionFile())
	}
	return files
}

// isBareName reports whether p is a program name to look up on PATH.
func isBareName(p string) bool {
	return !strings.ContainsAny(p, pathSeparators)
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
