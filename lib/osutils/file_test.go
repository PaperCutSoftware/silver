/*
 * Copyright © 2021 PaperCut Software International Pty. Ltd.
 */

package osutils_test

import (
	"github.com/papercutsoftware/silver/lib/osutils"
	"io/ioutil"
	"os"
	"testing"
)

func TestFileOps(t *testing.T) {
	file, err := ioutil.TempFile("", "testFileOps")
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	data := "A quick brown fox jumped over a lazy sleeping dog"
	err = osutils.WriteFileString(file.Name(), data, 0640)
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	if !osutils.FileExists(file.Name()) {
		t.Errorf("Expected file to exist: %s", file.Name())
	}

	if read := osutils.ReadStringFromFile(file.Name(), "default"); read != data {
		t.Errorf("Expected to read same data back\nWanted=%s\nGot=%s\n", data, read)
	}
}

func TestCopyFile(t *testing.T) {
	file, err := ioutil.TempFile("", "testCopyFile")
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	type args struct {
		src  string
		dest string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test copy non existing file",
			args: args{
				src:  "not-existing",
				dest: "",
			},
			wantErr: true,
		},
		{
			name: "Test copy existing file",
			args: args{
				src:  file.Name(),
				dest: file.Name() + ".copy",
			},
			wantErr: false,
		},
		{
			name: "Test copy to invalid destination",
			args: args{
				src:  file.Name(),
				dest: "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := osutils.CopyFile(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("CopyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadStringFromFile(t *testing.T) {
	file, err := ioutil.TempFile("", "testReadStringFromFile")
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	defer func() {
		_ = os.Remove(file.Name())
	}()
	_ = osutils.WriteFileString(file.Name(), "Test-1234", 0600)

	type args struct {
		file string
		def  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test read data",
			args: args{
				file: file.Name(),
				def:  "",
			},
			want: "Test-1234",
		},
		{
			name: "Test read invalid path",
			args: args{
				file: "not-existing",
				def:  "fallback",
			},
			want: "fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := osutils.ReadStringFromFile(tt.args.file, tt.args.def); got != tt.want {
				t.Errorf("ReadStringFromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
