/*
 * Copyright © 2021 PaperCut Software International Pty. Ltd.
 */

package updaterconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/papercutsoftware/silver/lib/osutils"
	"github.com/papercutsoftware/silver/service/config"
)

const (
	startupTasksKey   = "StartupTasks"
	scheduledTasksKey = "ScheduledTasks"
	pathKey           = "Path"
)

type UpdaterConf struct {
	silverDir            string      // The absolute path to the silver executable
	silverConfigFilename string      // the filename for the silver config file
	updaterFilename      string      // the filename without extension for the updater executable
	versionFilename      string      // Filename storing version information .version by default
	backupFilePrefix     string      // The prefix to use for backup files
	perm                 os.FileMode // Permission to use for all created files
	lock                 sync.Mutex
}

// Create creates UpdaterConf based on service location parameters.
func Create(
	silverDir string,
	silverConfigFilename string,
	updaterFilename string,
) (*UpdaterConf, error) {
	if silverDir == "" || silverConfigFilename == "" {
		return nil, errors.New("missing Silver service arguments")
	}
	if updaterFilename == "" {
		updaterFilename = "updater"
	}

	u := &UpdaterConf{
		silverDir:            silverDir,
		silverConfigFilename: silverConfigFilename,
		updaterFilename:      updaterFilename,
		versionFilename:      ".version",
		backupFilePrefix:     "backup-",
		perm:                 0600,
	}

	err := u.deleteReloadFile()
	if err != nil {
		return nil, err
	}

	err = u.backupServiceConfigIfRequired()
	if err != nil {
		return nil, err
	}
	return u, nil
}

// WithBackupPrefix specifies prefix to use for any backup files created
func (u *UpdaterConf) WithBackupPrefix(prefix string) *UpdaterConf {
	u.backupFilePrefix = prefix
	return u
}

// WithPermMode assigns desired permission mask to any files created, 0600 by default.
func (u *UpdaterConf) WithPermMode(perm os.FileMode) *UpdaterConf {
	u.perm = perm
	return u
}

// WithVersionFileName sets filename that stores the current update version, ".version" by default.
func (u *UpdaterConf) WithVersionFileName(versionFileName string) *UpdaterConf {
	u.versionFilename = versionFileName
	return u
}

// CurrentVersion reads the current update version from file
func (u *UpdaterConf) CurrentVersion() string {
	return osutils.ReadStringFromFile(u.getFilePath(u.versionFilename), "")
}

// IsReloading returns if the reloading flag been set to instruct service reload in progress.
func (u *UpdaterConf) IsReloading() bool {
	reloadFilePath := u.getReloadFilePath()
	return osutils.FileExists(reloadFilePath)
}

func (u *UpdaterConf) getFilePath(filename string) string {
	return filepath.Join(u.silverDir, filename)
}

func (u *UpdaterConf) getServiceConfigPath() string {
	return u.getFilePath(u.silverConfigFilename)
}

func (u *UpdaterConf) getBackupServiceConfigPath() string {
	return u.getFilePath(u.backupFilePrefix + u.silverConfigFilename)
}

func (u *UpdaterConf) getReloadFilePath() string {
	// No support for custom reload files in updater yet
	return u.getFilePath(config.ReloadFileName)
}

// EnableAutoUpdates enables auto updates
func (u *UpdaterConf) EnableAutoUpdates() error {
	u.lock.Lock()
	defer u.lock.Unlock()

	reloading := u.IsReloading()
	if reloading {
		return fmt.Errorf("app is already reloading, cannot enable auto updates until reloading complete")
	}
	enabled, err := u.IsAutoUpdateEnabled()
	if err != nil {
		return fmt.Errorf("unable to determine if updates already enabled: %w", err)
	} else if enabled {
		fmt.Printf("auto update already enabled\n")
		return nil
	}

	fmt.Printf("enabling auto updates\n")
	err = osutils.CopyFile(u.getBackupServiceConfigPath(), u.getServiceConfigPath())
	if err != nil {
		fmt.Printf("enableUpdates could not copy file: %v", err)
		return err
	}
	err = u.reloadApp()
	if err != nil {
		fmt.Printf("enableUpdates reloadApp error: %v\n", err)
		return err
	}
	fmt.Printf("auto updates enabled\n")
	return nil
}

// DisableAutoUpdate disables auto updates, backing up the active configuration file
func (u *UpdaterConf) DisableAutoUpdate() error {
	u.lock.Lock()
	defer u.lock.Unlock()

	reloading := u.IsReloading()
	if reloading {
		return fmt.Errorf("app is already reloading, cannot disable auto updates until reloading complete")
	}
	enabled, err := u.IsAutoUpdateEnabled()
	if err != nil {
		return fmt.Errorf("unable to determine if updates already enabled: %w", err)
	} else if !enabled {
		fmt.Printf("auto update already disabled\n")
		return nil
	}
	fmt.Printf("disabling auto updates\n")
	err = u.backupServiceConfigIfRequired()
	if err != nil {
		fmt.Printf("disableUpdates could not create default update config: %v\n", err)
		return err
	}
	conf, err := u.loadConfig(u.getServiceConfigPath())
	if err != nil {
		fmt.Printf("disableUpdates could not load config file: %v\n", err)
		return err
	}

	if u.removeUpdaterTasks(conf, startupTasksKey) {
		delete(conf, startupTasksKey)
	}
	if u.removeUpdaterTasks(conf, scheduledTasksKey) {
		delete(conf, scheduledTasksKey)
	}

	err = u.saveConfig(conf)
	if err != nil {
		fmt.Printf("disableUpdates saveUpdaterConfig error: %v\n", err)
		return err
	}
	err = u.reloadApp()
	if err != nil {
		fmt.Printf("disableUpdates reloadApp error: %v\n", err)
		return err
	}
	fmt.Printf("auto updates disabled\n")
	return nil
}

func (u *UpdaterConf) removeUpdaterTasks(conf map[string]interface{}, key string) bool {
	tasks, ok := conf[key].([]interface{})
	if !ok {
		return false
	}
	for i := 0; i < len(tasks); i++ {
		task, ok := tasks[i].(map[string]interface{})
		if !ok {
			continue
		}
		path, ok := task[pathKey].(string)
		if !ok {
			continue
		}
		if strings.Contains(path, u.updaterFilename) {
			tasks = append(tasks[:i], tasks[i+1:]...)
			continue
		}
	}
	conf[key] = tasks

	return len(tasks) == 0
}

// IsAutoUpdateEnabled checks if the auto update is currently enabled
func (u *UpdaterConf) IsAutoUpdateEnabled() (bool, error) {
	tasks, err := u.getTasksFromServiceConfig()
	if err != nil {
		return false, err
	}
	return u.containsUpdaterTask(tasks), nil
}

func (u *UpdaterConf) containsUpdaterTask(tasks []config.Task) bool {
	for _, task := range tasks {
		if u.isUpdaterTask(task) {
			return true
		}
	}
	return false
}

func (u *UpdaterConf) getTasksFromServiceConfig() ([]config.Task, error) {
	conf, err := config.LoadConfigNoReplacements(u.getServiceConfigPath())
	if err != nil {
		return nil, err
	}
	return u.extractTasks(conf), nil
}

func (u *UpdaterConf) isUpdaterTask(task config.Task) bool {
	return strings.Contains(task.Path, u.updaterFilename)
}

func (u *UpdaterConf) extractTasks(c *config.Config) []config.Task {
	var tasks []config.Task
	for _, t := range c.ScheduledTasks {
		tasks = append(tasks, t.Task)
	}
	for _, t := range c.StartupTasks {
		tasks = append(tasks, t.Task)
	}
	return tasks
}

func (u *UpdaterConf) loadConfig(filePath string) (map[string]interface{}, error) {
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var dto map[string]interface{}
	err = json.Unmarshal(bytes, &dto)
	if err != nil {
		return nil, fmt.Errorf("loadConfig failed to unmarshal JSON config file: %w", err)
	}
	return dto, nil
}

func (u *UpdaterConf) saveConfig(dto map[string]interface{}) error {
	file, err := json.MarshalIndent(dto, "", "    ")
	if err != nil {
		return fmt.Errorf("saveConfig failed to marshal JSON config data: %w", err)
	}
	filePath := u.getServiceConfigPath()
	return ioutil.WriteFile(filePath, file, u.perm)
}

func (u *UpdaterConf) deleteReloadFile() error {
	reloadFilePath := u.getReloadFilePath()

	if !osutils.FileExists(reloadFilePath) {
		return nil
	}
	return os.Remove(reloadFilePath)
}

// creates a backup config file for reference
func (u *UpdaterConf) backupServiceConfigIfRequired() error {
	destinationFile := u.getBackupServiceConfigPath()
	if osutils.FileExists(destinationFile) {
		return nil
	}
	sourceFile := u.getServiceConfigPath()
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return err
	}
	err = osutils.WriteFileString(destinationFile, string(input), u.perm)
	if err != nil {
		return err
	}
	return nil
}

// reloadApp triggers a service reload
func (u *UpdaterConf) reloadApp() error {
	filePath := u.getReloadFilePath()
	return ioutil.WriteFile(filePath, []byte{}, u.perm)
}
