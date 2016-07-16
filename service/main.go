// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kardianos/service"

	"github.com/robfig/cron"

	"github.com/papercutsoftware/silver/lib/logging"
	"github.com/papercutsoftware/silver/lib/pathutils"
	"github.com/papercutsoftware/silver/service/cmdutil"
	"github.com/papercutsoftware/silver/service/config"
	"github.com/papercutsoftware/silver/service/svcutil"
)

const (
	defaultRefreshPoll = 10 * time.Second
)

var (
	logger *log.Logger
	// FIXME: Remove globals!
	conf        *config.Config
	terminate   chan struct{}
	done        sync.WaitGroup
	cronManager *cron.Cron
)

func main() {

	// Parse config (we don't action any errors quite yet)
	conf, err := loadConf()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid config - %v\n", err)
		os.Exit(1)
	}

	action, actionArgs, err := parse(os.Args)
	if err != nil {
		printUsage(conf.ServiceDescription.DisplayName, conf.ServiceDescription.Description)
		os.Exit(1)
	}

	if err := os.Chdir(exeFolder()); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unable to set working directory - %v\n", err)
		os.Exit(1)
	}

	switch action {
	case "command":
		var cmdName string
		var cmdExtraArgs []string
		if len(actionArgs) >= 3 {
			cmdName = actionArgs[2]
		}
		if len(actionArgs) >= 4 {
			cmdExtraArgs = actionArgs[3:]
		}
		execCommand(cmdName, cmdExtraArgs)
	case "validate":
		fmt.Println("Config is valid")
		os.Exit(0)
	default:
		serviceControl(conf)
	}
}

func serviceControl(config *config.Config) {
	// Setup log file out
	logFile := conf.ServiceConfig.LogFile
	maxSize := int64(conf.ServiceConfig.LogFileMaxSizeMb) * 1024 * 1024
	if logFile == "" {
		logFile = serviceName() + ".log"
	}
	logger = logging.NewFileLoggerWithMaxSize(logFile, maxSize)

	// Setup service
	svcConfig := &service.Config{
		Name:        serviceName(),
		DisplayName: conf.ServiceDescription.DisplayName,
		Description: conf.ServiceDescription.Description,
	}

	prog := &program{}
	svc, err := service.New(prog, svcConfig)
	if err != nil {
		fmt.Printf("ERROR: Invalid service config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] != "run" {
		err = service.Control(svc, os.Args[1])
		if err != nil {
			fmt.Printf("ERROR: Invalid service command: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	err = svc.Run()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	pidFile := conf.ServiceConfig.PidFile
	if pidFile != "" {
		ioutil.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
	}
}

func printUsage(svcDisplayName, svcDesc string) {
	fmt.Printf("%s (%s)\n", svcDisplayName,
		serviceName())
	fmt.Printf("%s\n\n", svcDesc)
	fmt.Printf("Usage:\n")
	fmt.Printf("%s [install|uninstall|start|stop|command|validate|run|help] [command-name]\n", exeName())
	fmt.Printf("  install   - Install the service.\n")
	fmt.Printf("  uninstall - Remove/uninstall the service.\n")
	fmt.Printf("  start     - Start an installed service.\n")
	fmt.Printf("  stop      - Stop an installed service.\n")
	fmt.Printf("  validate  - Test the configuration file.\n")
	fmt.Printf("  run       - Run service on in command-line mode.\n")
	fmt.Printf("  command   - Run a command [command-name].\n")
	fmt.Printf("  help      - This usage message.\n")
}

func loadConf() (conf *config.Config, err error) {
	// FIXME: Not Get this function out of utils.
	confPath := getConfigFilePath()
	vars := config.ReplacementVars{
		ServiceName: serviceName(),
		ServiceRoot: exeFolder(),
	}
	conf, err = config.LoadConfig(confPath, vars)
	if err != nil {
		return nil, err
	}

	// Merge in any include files
	for _, include := range conf.Include {
		conf, err = config.MergeInclude(*conf, include, vars)
		if err != nil {
			return nil, err
		}
	}
	return conf, err
}

func execCommand(cmdName string, cmdExtraArgs []string) {
	if len(conf.Commands) == 0 {
		fmt.Fprintf(os.Stderr, "There are no commands configured!\n")
		os.Exit(1)
	}
	var cmd *config.Command
	for _, c := range conf.Commands {
		if c.Name == cmdName {
			cmd = &c
			break
		}
	}

	if cmd == nil {
		fmt.Fprintf(os.Stderr, "Valid commands are:\n")
		for _, command := range conf.Commands {
			fmt.Fprintf(os.Stderr, "    %s\n", command.Name)
		}
		os.Exit(1)
	}

	cmdConf := cmdutil.CommandConfig{}
	cmdConf.Path = pathutils.FindLastFile(cmd.Path)
	cmdConf.Args = cmd.Args
	cmdConf.ExecTimeout = (time.Second * time.Duration(cmd.TimeoutSecs))

	exitCode, err := cmdutil.Execute(cmdConf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	}
	os.Exit(exitCode)
}

type program struct{}

func (p *program) Start(s service.Service) error {
	msg := fmt.Sprintf("Service '%s' started.", serviceName())
	logger.Printf(msg)
	sysLogger, err := s.Logger(nil)
	if err != nil {
		sysLogger.Info(msg)
	}

	doStart()

	go watchForReload()
	return nil
}

func (p *program) Stop(s service.Service) error {
	logger.Printf(fmt.Sprintf("Stopping '%s' service...", serviceName()))

	doStop()

	pidFile := conf.ServiceConfig.PidFile
	if pidFile != "" {
		os.Remove(pidFile)
	}

	msg := fmt.Sprintf("Stopped '%s' service.", serviceName())
	logger.Printf(msg)

	sysLogger, err := s.Logger(nil)
	if err != nil {
		sysLogger.Info(msg)
	}
	return nil
}

func doStart() {
	terminate = make(chan struct{})
	execStartupTasks()
	setupScheduledTasks()
	startServices()
}

func doStop() {
	// Create stop file... another method to signal services to stop.
	stopFile := conf.ServiceConfig.StopFile
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
	f := conf.ServiceConfig.ReloadFile
	if f == "" {
		f = ".reload"
	}
	if f == "disabled" {
		return
	}
	for {
		// FIXME: File system notification rather than polling?
		time.Sleep(defaultRefreshPoll)
		if _, err := os.Stat(f); err == nil {
			if err := os.Remove(f); err == nil {
				logger.Printf("Reload requested")
				doStop()
				time.Sleep(time.Second)
				conf, _ = loadConf()
				doStart()
			}
		}
	}
}

func execStartupTasks() {
	for _, task := range conf.StartupTasks {
		runTask := func(task config.StartupTask) {
			done.Add(1)
			defer done.Done()
			taskConfig := createTaskConfig(task.Task)
			if exitCode, err := svcutil.ExecuteTask(terminate, taskConfig); err != nil {
				logger.Printf("ERROR: Startup task '%s' reported: %v", taskConfig.Path, err)
			} else {
				logger.Printf("The task exits with exit code %d", exitCode)
			}
		}
		if task.Async {
			if task.StartupDelaySecs > 0 || task.StartupRandomDelaySecs > 0 {
				logger.Printf("WARNING: Only Async startup tasks may have startup delays.")
			}
			go runTask(task)
		} else {
			runTask(task)
		}
	}
}

func startServices() {
	for _, service := range conf.Services {
		go func(service config.Service) {
			done.Add(1)
			defer done.Done()
			svcConfig := svcutil.ServiceConfig{}
			svcConfig.Path = service.Path
			svcConfig.Args = service.Args
			svcConfig.GracefulShutDown = time.Duration(service.GracefulShutdownTimeout) * time.Second
			svcConfig.StartupDelay = time.Duration(service.StartupDelaySecs) * time.Second
			svcConfig.Logger = logger
			svcConfig.CrashConfig = svcutil.CrashConfig{
				MaxCount:     service.MaxCrashCount,
				RestartDelay: time.Duration(service.RestartDelaySecs) * time.Second,
			}
			svcConfig.MonitorConfig = svcutil.MonitorConfig{
				URL:                   service.MonitorPing.URL,
				StartupDelay:          time.Duration(service.MonitorPing.StartupDelaySecs) * time.Second,
				Interval:              time.Duration(service.MonitorPing.IntervalSecs) * time.Second,
				Timeout:               time.Duration(service.MonitorPing.TimeoutSecs) * time.Second,
				RestartOnFailureCount: service.MonitorPing.RestartOnFailureCount,
			}
			if err := svcutil.ExecuteService(terminate, svcConfig); err != nil {
				logger.Printf("ERROR: Service '%s' reported: %v", service.Path, err)

			}
		}(service)
	}
}

func createTaskConfig(task config.Task) svcutil.TaskConfig {
	taskConfig := svcutil.TaskConfig{}
	taskConfig.Path = pathutils.FindLastFile(task.Path)
	taskConfig.Args = task.Args
	taskConfig.ExecTimeout = time.Duration(task.TimeoutSecs) * time.Second
	taskConfig.StartupDelay = time.Duration(task.StartupDelaySecs) * time.Second
	taskConfig.StartupRandomDelay = time.Duration(task.StartupRandomDelaySecs) * time.Second
	taskConfig.Logger = logger // TODO: Global?????
	return taskConfig
}

func setupScheduledTasks() {
	cronManager = cron.New()
	for _, scheduledTask := range conf.ScheduledTasks {
		taskConfig := createTaskConfig(scheduledTask.Task)
		runTask := func() {
			done.Add(1)
			defer done.Done()
			if exitCode, err := svcutil.ExecuteTask(terminate, taskConfig); err != nil {
				logger.Printf("ERROR: scheduled task '%s' reported: %v", taskConfig.Path, err)
			} else {
				logger.Printf("The task exits with exit code %d", exitCode)
			}
		}
		err := cronManager.AddFunc(scheduledTask.Schedule, runTask)
		if err != nil {
			logger.Printf("Unable to schedule task '%s': %v", scheduledTask.Path, err)
		}
	}
	cronManager.Start()
}
