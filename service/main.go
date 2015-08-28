// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"bitbucket.org/kardianos/service/stdservice"
	"github.com/robfig/cron"

	"github.com/papercutsoftware/silver/lib/logging"
	"github.com/papercutsoftware/silver/lib/pathutils"
	"github.com/papercutsoftware/silver/lib/run"
)

const (
	defaultRefreshPollSecs = 10
)

var (
	logger      *log.Logger
	config      *Config
	configErr   error
	terminate   chan struct{}
	done        sync.WaitGroup
	cronManager *cron.Cron
)

func main() {

	// Parse config (we don't action any errors quite yet)
	config, configErr = LoadConfig()
	svcDisplayName := ""
	svcDescription := ""
	if configErr == nil {
		svcDisplayName = config.ServiceDescription.DisplayName
		svcDescription = config.ServiceDescription.Description
	}

	normalizeArgs()

	// Check args
	if !validateArgs() {
		fmt.Printf("%s (%s)\n", svcDisplayName,
			serviceName())
		fmt.Printf("%s\n\n", svcDescription)
		fmt.Printf("Usage:\n")
		fmt.Printf("%s [install|remove|start|stop|command|validate|run|help] [command-name]\n", exeName())
		fmt.Printf("  install   - Install the service.\n")
		fmt.Printf("  remove    - Remove/uninstall the service.\n")
		fmt.Printf("  start     - Start an installed service.\n")
		fmt.Printf("  stop      - Stop an installed service.\n")
		fmt.Printf("  validate  - Test the configuration file.\n")
		fmt.Printf("  run       - Run service on in command-line mode.\n")
		fmt.Printf("  command   - Run a command [command-name].\n")
		fmt.Printf("  help      - This usage message.\n")
		os.Exit(1)
	}

	if err := os.Chdir(exeFolder()); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unable to set working directory - %v\n", err)
		os.Exit(1)
	}

	// Run command if requested
	if len(os.Args) > 1 && (os.Args[1] == "command" ||
		os.Args[1] == "test" || os.Args[1] == "validate") {
		if configErr != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Invalid config - %v\n", configErr)
			os.Exit(1)
		}
		if os.Args[1] == "command" {
			execCommand()
		} else {
			fmt.Println("Config is valid")
			os.Exit(0)
		}
		return
	}

	// Setup log file out
	logFile := ""
	maxSize := int64(0)
	if configErr == nil {
		logFile = config.ServiceConfig.LogFile
		maxSize = int64(config.ServiceConfig.LogFileMaxSizeMb) * 1024 * 1024
	}
	if logFile == "" {
		logFile = serviceName() + ".log"
	}
	logger = logging.NewFileLoggerWithMaxSize(logFile, maxSize)

	// Run service
	srvConfig := &stdservice.Config{
		Name:            serviceName(),
		DisplayName:     svcDisplayName,
		LongDescription: svcDescription,

		Start: onServiceStart,
		Init:  onServiceInit,
		Stop:  onServiceStop,
	}
	srvConfig.Run()
}

func execCommand() {
	requestedCmd := ""
	if len(os.Args) > 2 {
		requestedCmd = os.Args[2]
	}
	var cmd *Command
	for _, c := range config.Commands {
		if c.Name == requestedCmd {
			cmd = c
			break
		}
	}

	if cmd == nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unknown command '%s'. ", requestedCmd)
		if len(config.Commands) == 0 {
			fmt.Fprintf(os.Stderr, "There are no commands configured!\n")
		} else {
			fmt.Fprintf(os.Stderr, "Valid commands are:\n")
			for _, cmd = range config.Commands {
				fmt.Fprintf(os.Stderr, "    %s\n", cmd.Name)
			}
		}
		os.Exit(1)
	}

	// Exec the command
	c := new(run.CommandConfig)
	c.Path = pathutils.FindLastFile(cmd.Path)
	if len(os.Args) > 3 {
		c.Args = append(cmd.Args, os.Args[3:]...)
	} else {
		c.Args = cmd.Args
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Logger = nil // No logger as we're in console mode (e.g. end-user)

	exitCode, err := run.RunCommand(c, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	}
	os.Exit(exitCode)
}

func onServiceInit(c *stdservice.Config) error {
	// Report config error here so it's logged.
	if configErr != nil {
		msg := fmt.Sprintf("ERROR: Invalid service config - '%v'\n", configErr)
		logger.Printf(msg)
		return errors.New(msg) // This will log at the service event layer
	}

	pidFile := config.ServiceConfig.PidFile
	if pidFile != "" {
		ioutil.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
	}

	return nil
}

func onServiceStart(c *stdservice.Config) {
	msg := fmt.Sprintf("Service '%s' started.", serviceName())
	logger.Printf(msg)
	c.Logger().Info(msg)

	doStart()

	go watchForReload()

	// onService should never return
	select {}
}

func onServiceStop(c *stdservice.Config) {
	logger.Printf(fmt.Sprintf("Stopping '%s' service...", serviceName()))

	doStop()

	pidFile := config.ServiceConfig.PidFile
	if pidFile != "" {
		os.Remove(pidFile)
	}

	msg := fmt.Sprintf("Stopped '%s' service.", serviceName())
	logger.Printf(msg)
	c.Logger().Info(msg)
}

func doStart() {
	terminate = make(chan struct{})
	execStartupTasks()
	setupScheduledTasks()
	startServices()
}

func doStop() {
	// Create stop file... another method to signal services to stop.
	stopFile := config.ServiceConfig.StopFile
	if stopFile == "" {
		stopFile = ".stop"
	}
	if stopFile == "disabled" {
		return
	}
	ioutil.WriteFile(stopFile, nil, 0644)
	defer os.Remove(stopFile)
	if cronManager != nil {
		cronManager.Stop()
	}
	if terminate != nil {
		close(terminate)
	}
	done.Wait()
}

func watchForReload() {
	f := config.ServiceConfig.ReloadFile
	if f == "" {
		f = ".reload"
	}
	if f == "disabled" {
		return
	}
	for {
		// FIXME: File system notification rather than polling?
		time.Sleep(defaultRefreshPollSecs * time.Second)
		if _, err := os.Stat(f); err == nil {
			if err := os.Remove(f); err == nil {
				logger.Printf("Reload requested")
				doStop()
				time.Sleep(time.Second)
				config, _ = LoadConfig()
				doStart()
			}
		}
	}
}

func execStartupTasks() {
	for _, task := range config.StartupTasks {
		runTask := func() {
			defer done.Done()
			done.Add(1)

			cc := new(run.CommandConfig)
			cc.Path = pathutils.FindLastFile(task.Path)
			cc.Args = task.Args
			cc.Logger = logger
			cc.TimeoutSecs = task.TimeoutSecs
			if task.Async {
				cc.StartupDelaySecs = task.StartupDelaySecs
				cc.StartupRandomDelaySecs = task.StartupRandomDelaySecs
			} else if cc.StartupDelaySecs > 0 || cc.StartupRandomDelaySecs > 0 {
				logger.Printf("WARNING: Only Async startup tasks may have startup delays.")
			}
			if _, err := run.RunCommand(cc, terminate); err != nil {
				logger.Printf("ERROR: Startup task '%s' reported: %v", cc.Path, err)
			}
		}
		if task.Async {
			go runTask()
		} else {
			runTask()
		}
	}
}

func startServices() {
	for _, service := range config.Services {
		runService := func() {
			defer done.Done()
			done.Add(1)

			sc := new(run.ServiceConfig)
			sc.Path = pathutils.FindLastFile(service.Path)
			sc.Args = service.Args
			sc.Logger = logger
			sc.MaxCrashCount = service.MaxCrashCount
			sc.RestartDelaySecs = service.RestartDelaySecs
			sc.StartupDelaySecs = service.StartupDelaySecs

			if err := run.RunService(sc, terminate); err != nil {
				logger.Printf("ERROR: Service '%s' reported: %v", sc.Path, err)
			}
		}
		go runService()
	}
}

func setupScheduledTasks() {
	cronManager = cron.New()
	for _, task := range config.ScheduledTasks {
		cc := new(run.CommandConfig)
		cc.Path = pathutils.FindLastFile(task.Path)
		cc.Args = task.Args
		cc.Logger = logger
		cc.TimeoutSecs = task.TimeoutSecs
		cc.StartupDelaySecs = task.StartupDelaySecs
		cc.StartupRandomDelaySecs = task.StartupRandomDelaySecs
		runTask := func() {
			defer done.Done()
			done.Add(1)
			if _, err := run.RunCommand(cc, terminate); err != nil {
				logger.Printf("Error raised by scheduled task '%s': %v", cc.Path, err)
			}
		}
		err := cronManager.AddFunc(task.Schedule, runTask)
		if err != nil {
			logger.Printf("Unable to schedule task '%s': %v", task.Path, err)
		}
	}
	cronManager.Start()
}

func normalizeArgs() {
	if len(os.Args) <= 1 {
		return
	}

	// Strip off any off the standard prefixes on first arg
	os.Args[1] = strings.TrimLeft(os.Args[1], "-/")

	// Setup a few 1st command aliases
	aliases := map[string]string{
		"setup":     "install",
		"uninstall": "remove",
		"delete":    "remove",
		"check":     "validate",
	}
	if alias, ok := aliases[os.Args[1]]; ok {
		os.Args[1] = alias
	}
}

func validateArgs() bool {
	validArgs := [...]string{
		"install",
		"remove",
		"start",
		"stop",
		"validate",
		"run",
		"command",
	}
	if len(os.Args) < 2 {
		// No command to validate
		return false
	}

	for _, arg := range validArgs {
		if arg == os.Args[1] {
			return true
		}
	}
	return false
}
