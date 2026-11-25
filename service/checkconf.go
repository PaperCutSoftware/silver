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

	"github.com/papercutsoftware/silver/lib/pathutils"
	"github.com/papercutsoftware/silver/service/config"
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
		c, _, ok := componentOf(p)
		if !ok {
			continue
		}
		if got, want := c.versionOf(resolved), current[c]; want != "" && got != want {
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

// currentVersionFolders maps each component to its current version folder.
// Config include globs decide it first; a component
// that runnable paths use but no include covers falls back to its newest
// folder, or "" if it has none.
//
// if includes of the same component resolve to different folders, returns
// an error
// e.g.
// Include: ["component1/v*/component1.conf", "component2/v*/component2.conf"]
// -> {"<root>/component1": "v124", "<root>/component2": "v7"}
func currentVersionFolders(conf *config.Config) (map[component]string, error) {
	current := map[component]string{}
	for _, inc := range conf.Include {
		c, _, ok := componentOf(inc)
		if !ok {
			continue
		}
		resolved := pathutils.FindLastFile(inc)
		if resolved == inc {
			continue // no match
		}
		folder := c.versionOf(resolved)
		if prev, ok := current[c]; ok && prev != folder {
			return nil, fmt.Errorf("%s: resolved to folder '%s', but the component's earlier includes resolved to '%s'", inc, folder, prev)
		}
		current[c] = folder
	}
	for _, p := range runnablePaths(conf) {
		c, glob, ok := componentOf(p)
		if !ok {
			continue
		}
		if _, ok := current[c]; !ok {
			current[c] = c.newestFolder(glob)
		}
	}
	return current, nil
}
