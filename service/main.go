// SILVER - Service Wrapper
//
// Copyright (c) 2014-2017 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kardianos/service"
	"github.com/robfig/cron"

	"github.com/papercutsoftware/silver/lib/logging"
	"github.com/papercutsoftware/silver/lib/osutils"
	"github.com/papercutsoftware/silver/lib/pathutils"
	"github.com/papercutsoftware/silver/service/cmdutil"
	"github.com/papercutsoftware/silver/service/config"
	"github.com/papercutsoftware/silver/service/svcutil"
)

const (
	defaultRefreshPoll = 10 * time.Second
)

type context struct {
	conf         *config.Config
	terminate    chan struct{}
	logger       *log.Logger
	runningGroup sync.WaitGroup
	cronManager  *cron.Cron
}

func main() {

	if err := os.Chdir(exeFolder()); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unable to set working directory: %v\n", err)
		os.Exit(1)
	}

	ctx := &context{}

	// Parse config (we don't action any errors quite yet)
	var err error
	ctx.conf, err = loadConf()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid config - %v\n", err)
		os.Exit(1)
	}

	action, actionArgs, err := parse(os.Args)
	if err != nil {
		printUsage(ctx.conf.ServiceDescription.DisplayName, ctx.conf.ServiceDescription.Description)
		os.Exit(1)
	}

	setupEnvironment(ctx.conf)

	switch action {
	case "command":
		execCommand(ctx, actionArgs)
	case "validate":
		fmt.Println("Config is valid")
		os.Exit(0)
	case "install":
		if err := writeProxyConf(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Unable to store HTTP Proxy settings: %v\n", err)
		}
		fallthrough
	default:
		osServiceControl(ctx)
	}
}

func osServiceControl(ctx *context) {
	// Setup log file out
	logFile := ctx.conf.ServiceConfig.LogFile
	maxSize := int64(ctx.conf.ServiceConfig.LogFileMaxSizeMb) * 1024 * 1024
	if logFile == "" {
		logFile = serviceName() + ".log"
	}
	ctx.logger = logging.NewFileLoggerWithMaxSize(logFile, ctx.conf.ServiceUserName, maxSize)

	// Setup service
        svcConfig := &service.Config{
                Name:        serviceName(),
                DisplayName: ctx.conf.ServiceDescription.DisplayName,
                Description: ctx.conf.ServiceDescription.Description,
                UserName:    ctx.conf.ServiceUserName,
        }

	osService := &osService{ctx: ctx}
	svc, err := service.New(osService, svcConfig)
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

	pidFile := ctx.conf.ServiceConfig.PidFile
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
		include = pathutils.FindLastFile(include)
		conf, err = config.MergeInclude(*conf, include, vars)
		if err != nil {
			return nil, err
		}
	}
	return conf, err
}

func setupEnvironment(conf *config.Config) {
	// Load Silver spacific
	os.Setenv("SILVER_SERVICE_NAME", conf.ServiceDescription.Name)
	os.Setenv("SILVER_SERVICE_ROOT", exeFolder())
	os.Setenv("SILVER_SERVICE_PID", string(os.Getpid()))

	// If we have HTTP proxy conf, load this
	if b, err := ioutil.ReadFile(proxyConfFile()); err == nil {
		proxy := strings.TrimSpace(string(b))
		if proxy != "" {
			os.Setenv("SILVER_HTTP_PROXY", proxy)
		}
	}

	// Load any configured env
	for k, v := range conf.EnvironmentVars {
		os.Setenv(k, v)
	}
}

func writeProxyConf() error {
	proxy, err := osutils.GetHTTPProxy()
	if err != nil {
		proxy = ""
	}
	return ioutil.WriteFile(proxyConfFile(), []byte(proxy+"\n"), 0644)
}

func proxyConfFile() string {
	return filepath.Join(exeFolder(), "http-proxy.conf")
}

func execCommand(ctx *context, args []string) {
	/*
	*  IMPORTANT:
	*  Don't write to any log files, etc.  Commands are not system service code.
	*  Commands should be thought of as a "symlink" style, and run under a different
	*  user context.
	*
	*  args format: 1st element is the command. Any extras are appended to the command.
	 */
	if len(ctx.conf.Commands) == 0 {
		fmt.Fprintf(os.Stderr, "There are no commands configured!\n")
		os.Exit(1)
	}

	var cmd *config.Command
	if len(args) > 0 {
		cmdName := args[0]
		for _, c := range ctx.conf.Commands {
			if c.Name == cmdName {
				cmd = &c
				break
			}
		}
	}

	if cmd == nil {
		// Print command usage
		fmt.Fprintf(os.Stderr, "Valid commands are:\n")
		for _, command := range ctx.conf.Commands {
			fmt.Fprintf(os.Stderr, "    %s\n", command.Name)
		}
		os.Exit(1)
	}

	cmdConf := cmdutil.CommandConfig{}
	cmdConf.Path = pathutils.FindLastFile(cmd.Path)
	// Append any extra commands
	cmdConf.Args = append(cmd.Args, args[1:]...)
	// FIXME: Maybe unit conversion should be in the config layer?
	cmdConf.ExecTimeout = (time.Second * time.Duration(cmd.TimeoutSecs))

	exitCode, err := cmdutil.Execute(cmdConf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	}
	os.Exit(exitCode)
}

type osService struct {
	ctx *context
}

func (o *osService) Start(s service.Service) error {
	msg := fmt.Sprintf("Service '%s' started.", serviceName())
	o.ctx.logger.Printf(msg)
	sysLogger, err := s.Logger(nil)
	if err != nil {
		sysLogger.Info(msg)
	}

	proxy := os.Getenv("SILVER_HTTP_PROXY")
	if proxy != "" {
		o.ctx.logger.Printf("Proxy set to: '%s'", proxy)
	}

	doStart(o.ctx)
	go watchForReload(o.ctx)

	return nil
}

func doStart(ctx *context) {
	sf := stopFileName(ctx)
	if sf != "" {
		os.Remove(sf)
	}
	ctx.terminate = make(chan struct{})
	execStartupTasks(ctx)
	setupScheduledTasks(ctx)
	startServices(ctx)
}

func (o *osService) Stop(s service.Service) error {
	o.ctx.logger.Printf(fmt.Sprintf("Stopping '%s' service...", serviceName()))

	doStop(o.ctx)

	pidFile := o.ctx.conf.ServiceConfig.PidFile
	if pidFile != "" {
		os.Remove(pidFile)
	}

	msg := fmt.Sprintf("Stopped '%s' service.", serviceName())
	o.ctx.logger.Printf(msg)

	sysLogger, err := s.Logger(nil)
	if err != nil {
		sysLogger.Info(msg)
	}
	return nil
}

func stopFileName(ctx *context) string {
	stopFile := ctx.conf.ServiceConfig.StopFile
	if stopFile != "disabled" {
		return ""
	}
	return stopFile
}

func doStop(ctx *context) {
	// Create stop file... another method to signal services to stop.
	sf := stopFileName(ctx)
	if sf != "" {
		ioutil.WriteFile(sf, nil, 0644)
		defer os.Remove(sf)
	}
	if ctx.cronManager != nil {
		ctx.cronManager.Stop()
		ctx.cronManager = nil
	}
	if ctx.terminate != nil {
		close(ctx.terminate)
	}
	ctx.runningGroup.Wait()
}

func watchForReload(ctx *context) {
	f := ctx.conf.ServiceConfig.ReloadFile
	for {
		// FIXME: File system notification rather than polling?
		time.Sleep(defaultRefreshPoll)
		if _, err := os.Stat(f); err == nil {
			if err := os.Remove(f); err == nil {
				ctx.logger.Printf("Reload requested. Services will now restart.")
				doStop(ctx)
				time.Sleep(time.Second)
				// Reload config
				ctx.conf, _ = loadConf()
				doStart(ctx)
			}
		}
	}
}

func execStartupTasks(ctx *context) {
	ctx.logger.Printf("Starting %d startup tasks.", len(ctx.conf.StartupTasks))
	for _, task := range ctx.conf.StartupTasks {
		runTask := func(task config.StartupTask) {
			ctx.runningGroup.Add(1)
			defer ctx.runningGroup.Done()
			taskName := path.Base(task.Path)
			taskConfig := createTaskConfig(ctx, task.Task)
			if exitCode, err := svcutil.ExecuteTask(ctx.terminate, taskConfig); err != nil {
				ctx.logger.Printf("ERROR: Startup task '%s' reported: %v", taskName, err)
			} else {
				ctx.logger.Printf("Startup task '%s' finished with exit code %d", taskName, exitCode)
			}
		}
		if task.Async {
			go runTask(task)
		} else {
			if task.StartupDelaySecs > 0 || task.StartupRandomDelaySecs > 0 {
				ctx.logger.Printf("WARNING: Only Async startup tasks should have startup delays.")
			}
			runTask(task)
		}
	}
}

func startServices(ctx *context) {
	ctx.logger.Printf("Starting %d services.", len(ctx.conf.Services))
	for _, service := range ctx.conf.Services {
		go func(service config.Service) {
			ctx.runningGroup.Add(1)
			defer ctx.runningGroup.Done()
			serviceName := path.Base(service.Path)
			svcConfig := svcutil.ServiceConfig{}
			svcConfig.Path = pathutils.FindLastFile(service.Path)
			svcConfig.Args = service.Args
			svcConfig.GracefulShutDown = time.Duration(service.GracefulShutdownTimeoutSecs) * time.Second
			svcConfig.StartupDelay = time.Duration(service.StartupDelaySecs) * time.Second
			svcConfig.Logger = ctx.logger
			svcConfig.CrashConfig = svcutil.CrashConfig{
				MaxCountPerHour: service.MaxCrashCountPerHour,
				RestartDelay:    time.Duration(service.RestartDelaySecs) * time.Second,
			}
			if service.MonitorPing != nil {
				svcConfig.MonitorConfig = svcutil.MonitorConfig{
					URL:                   service.MonitorPing.URL,
					StartupDelay:          time.Duration(service.MonitorPing.StartupDelaySecs) * time.Second,
					Interval:              time.Duration(service.MonitorPing.IntervalSecs) * time.Second,
					Timeout:               time.Duration(service.MonitorPing.TimeoutSecs) * time.Second,
					RestartOnFailureCount: service.MonitorPing.RestartOnFailureCount,
				}
			}
			if err := svcutil.ExecuteService(ctx.terminate, svcConfig); err != nil {
				ctx.logger.Printf("ERROR: Service '%s' reported: %v", serviceName, err)
			}
		}(service)
	}
}

func createTaskConfig(ctx *context, task config.Task) svcutil.TaskConfig {
	taskConfig := svcutil.TaskConfig{}
	taskConfig.Path = pathutils.FindLastFile(task.Path)
	taskConfig.Args = task.Args
	taskConfig.ExecTimeout = time.Duration(task.TimeoutSecs) * time.Second
	taskConfig.StartupDelay = time.Duration(task.StartupDelaySecs) * time.Second
	taskConfig.StartupRandomDelay = time.Duration(task.StartupRandomDelaySecs) * time.Second
	taskConfig.Logger = ctx.logger
	return taskConfig
}

func setupScheduledTasks(ctx *context) {
	ctx.logger.Printf("Setting up %d scheduled tasks.", len(ctx.conf.ScheduledTasks))
	ctx.cronManager = cron.New()
	for _, scheduledTask := range ctx.conf.ScheduledTasks {
		taskConfig := createTaskConfig(ctx, scheduledTask.Task)
		runTask := func() {
			ctx.runningGroup.Add(1)
			defer ctx.runningGroup.Done()
			taskName := path.Base(taskConfig.Path)
			ctx.logger.Printf("Running schedule task '%s'", taskName)
			if exitCode, err := svcutil.ExecuteTask(ctx.terminate, taskConfig); err != nil {
				ctx.logger.Printf("ERROR: Scheduled task '%s' reported: %v", taskName, err)
			} else {
				ctx.logger.Printf("The task '%s' finished with exit code %d", taskName, exitCode)
			}
		}
		err := ctx.cronManager.AddFunc(scheduledTask.Schedule, runTask)
		if err != nil {
			ctx.logger.Printf("Unable to schedule task '%s': %v", scheduledTask.Path, err)
		}
	}
	ctx.cronManager.Start()
}
