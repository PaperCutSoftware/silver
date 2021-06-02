// SILVER - Service Wrapper
//
// Copyright (c) 2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package updaterconf_test

import (
	"github.com/papercutsoftware/silver/lib/osutils"
	"github.com/papercutsoftware/silver/service/config"
	"github.com/papercutsoftware/silver/service/updaterconf"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateDisableEnableRoundTrip(t *testing.T) {
	str := `{
	"ServiceDescription": {
        "DisplayName": "The Simple Service",
        "Description": "Simple service with update"
    },
    "Services": [
        {
            "Path": "simple-server.exe"
        }
    ],
	"StartupTasks": [
        {
            "Path": "my-updater.exe",
            "Args": ["https://my-update.acme.com/product-version/v1/check-update/"]
        }
	]
}`
	file, err := createConf(str)
	if err != nil {
		t.Errorf("Failed to create conf file: %v", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()

	silverDir := filepath.Dir(file.Name())
	updaterConf, err := updaterconf.Create(silverDir, file.Name(), "my-updater")
	if err != nil {
		t.Errorf("Failed to create updater: %v", err)
	}
	if !osutils.FileExists(filepath.Join(silverDir, "backup-"+filepath.Base(file.Name()))) {
		t.Errorf("Expected silver config file to be backed up")
	}

	if err = updaterConf.DisableAutoUpdate(); err != nil {
		t.Errorf("Failed to disable update: %v", err)
	}

	if enabled, err := updaterConf.IsAutoUpdateEnabled(); err != nil {
		t.Errorf("Failed to check if update is enabled: %v", err)
	} else if enabled {
		t.Errorf("Expected update to be disabled")
	}

	if conf, err := config.LoadConfigNoReplacements(file.Name()); err != nil {
		t.Errorf("Failed to load updated config: %v", err)
	} else if len(conf.StartupTasks) > 0 {
		t.Errorf("Expected all updated tasks to be removed")
	}
	_ = os.Remove(filepath.Join(silverDir, config.ReloadFileName))

	if err = updaterConf.EnableAutoUpdates(); err != nil {
		t.Errorf("Failed to enable update: %v", err)
	}

	if enabled, err := updaterConf.IsAutoUpdateEnabled(); err != nil {
		t.Errorf("Failed to check if update is enabled: %v", err)
	} else if !enabled {
		t.Errorf("Expected update to be enabled")
	}
}

func TestConfWithoutUpdates(t *testing.T) {
	str := `{
	"ServiceDescription": {
        "DisplayName": "The Simple Service",
        "Description": "Simple service without update"
    },
    "Services": [
        {
            "Path": "simple-server.exe"
        }
    ],
	"StartupTasks": [
        {
            "Path": "not-updater.exe",
            "Args": ["foo"]
        }
	]
}`
	file, err := createConf(str)
	if err != nil {
		t.Errorf("Failed to create conf file: %v", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()

	silverDir := filepath.Dir(file.Name())
	updaterConf, err := updaterconf.Create(silverDir, file.Name(), "my-updater")
	if err != nil {
		t.Errorf("Failed to create updater: %v", err)
	}

	if enabled, err := updaterConf.IsAutoUpdateEnabled(); err != nil {
		t.Errorf("Failed to check if update is enabled: %v", err)
	} else if enabled {
		t.Errorf("Expected update to be disabled")
	}

	if err = updaterConf.DisableAutoUpdate(); err != nil {
		t.Errorf("Failed to disable update: %v", err)
	}

	if conf, err := config.LoadConfigNoReplacements(file.Name()); err != nil {
		t.Errorf("Failed to load updated config: %v", err)
	} else if len(conf.StartupTasks) != 1 {
		t.Errorf("Expected all updated tasks to be left")
	}
}

func createConf(data string) (*os.File, error) {
	file, err := ioutil.TempFile("", "test-silver-config")
	if err != nil {
		return nil, err
	}
	err = osutils.WriteFileString(file.Name(), data, 0600)
	if err != nil {
		return nil, err
	}
	return file, err
}

func TestCreateArgs(t *testing.T) {
	file, err := createConf(`{"ServiceDescription": {"DisplayName": "The Simple Service"`)
	if err != nil {
		t.Errorf("Failed to create conf file: %v", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()

	type args struct {
		silverDir            string
		silverConfigFilename string
		updaterFilename      string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test updater create invalid args",
			args: args{
				silverDir:            "xyz",
				silverConfigFilename: "not-found",
				updaterFilename:      "foobar",
			},
			wantErr: true,
		},
		{
			name: "Test updater create invalid empty dir",
			args: args{
				silverDir:            "",
				silverConfigFilename: "not-found",
			},
			wantErr: true,
		},
		{
			name: "Test updater create invalid empty conf",
			args: args{
				silverDir: ".",
			},
			wantErr: true,
		},
		{
			name: "Test updater create with valid args",
			args: args{
				silverDir:            filepath.Dir(file.Name()),
				silverConfigFilename: filepath.Base(file.Name()),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updaterconf.Create(tt.args.silverDir, tt.args.silverConfigFilename, tt.args.updaterFilename)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got == nil {
				t.Errorf("Create expected to return non nil")
			}
		})
	}
}
