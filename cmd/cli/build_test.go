// Copyright (c) Huawei Technologies Co., Ltd. 2020. All rights reserved.
// isula-build licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: iSula Team
// Create: 2020-01-20
// Description: This file is used for building test

package main

import (
	"context"
	"crypto"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	constant "isula.org/isula-build"
	_ "isula.org/isula-build/exporter/register"
	"isula.org/isula-build/util"
)

func TestBuildCommand(t *testing.T) {
	dockerfile := ``
	filename := "Dockerfile"
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile(filename, dockerfile))
	defer tmpDir.Remove()

	type testcase struct {
		name      string
		file      string
		args      []string
		wanterr   bool
		errString string
	}
	var testcases = []testcase{
		{
			name:      "TC1 - normal case",
			file:      tmpDir.Path(),
			args:      []string{tmpDir.Path()},
			wanterr:   true,
			errString: "isula_build.sock",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			buildCmd := NewBuildCmd()
			err := buildCmd.Execute()
			assert.Equal(t, err != nil, true)

			err = buildCommand(buildCmd, tc.args)
			if tc.wanterr {
				assert.ErrorContains(t, err, tc.errString)
			}
			if !tc.wanterr {
				assert.NilError(t, err)
			}
		})
	}
}

func TestRunBuildWithLocalDockerfile(t *testing.T) {
	dockerfile := `
		FROM alpine:latest
		RUN echo hello world
		`
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile("Dockerfile", dockerfile))
	defer tmpDir.Remove()

	mockBuild := newMockDaemon()
	ctx := context.Background()
	cli := newMockClient(&mockGrpcClient{imageBuildFunc: mockBuild.build})

	buildOpts.file = tmpDir.Join("Dockerfile")
	var args []string
	err := newBuildOptions(args)
	assert.NilError(t, err)
	buildOpts.output = "docker-daemon:isula:latest"

	testcases := []struct {
		format string
	}{
		{
			format: "docker",
		},
		{
			format: "oci",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.format, func(t *testing.T) {
			buildOpts.format = tc.format
			_, err = runBuild(ctx, &cli)

			assert.NilError(t, err)
			assert.Equal(t, mockBuild.dockerfile(t), dockerfile)
			wd, err := os.Getwd()
			assert.NilError(t, err)
			realWd, err := filepath.EvalSymlinks(wd)
			assert.NilError(t, err)
			assert.Equal(t, mockBuild.contextDir(t), realWd)
		})
	}

}

func TestRunStatus(t *testing.T) {
	mockBuild := newMockDaemon()
	ctx := context.Background()
	cli := newMockClient(&mockGrpcClient{statusFunc: mockBuild.status})
	var args []string
	err := newBuildOptions(args)
	assert.NilError(t, err)
	buildOpts.buildID = "09f5f06de97cdf53d5d94cbb6a11e61b646ae4685ed003a752ebde352f09175a"
	err = runStatus(ctx, &cli)
	assert.NilError(t, err)
}

func TestRunBuildWithDefaultDockerFile(t *testing.T) {
	dockerfile := `
		FROM alpine:latest
		RUN echo hello world
		`
	wd, err := os.Getwd()
	assert.NilError(t, err)
	realWd, err := filepath.EvalSymlinks(wd)
	assert.NilError(t, err)
	filePath := filepath.Join(realWd, "Dockerfile")
	err = ioutil.WriteFile(filePath, []byte(dockerfile), constant.DefaultSharedFileMode)
	assert.NilError(t, err)
	defer func() {
		err = os.Remove(filePath)
		assert.NilError(t, err)
	}()

	mockBuild := newMockDaemon()
	ctx := context.Background()
	cli := newMockClient(&mockGrpcClient{imageBuildFunc: mockBuild.build})

	buildOpts.file = ""
	var args []string
	err = newBuildOptions(args)
	assert.NilError(t, err)
	buildOpts.output = "docker-daemon:isula:latest"

	testcases := []struct {
		format string
	}{
		{
			format: "docker",
		},
		{
			format: "oci",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.format, func(t *testing.T) {
			buildOpts.format = tc.format
			_, err = runBuild(ctx, &cli)

			assert.NilError(t, err)
			assert.Equal(t, mockBuild.dockerfile(t), dockerfile)
			assert.Equal(t, mockBuild.contextDir(t), realWd)
		})
	}
}

// Test runBuild with non archive exporter
// case 1. docker-daemon exporter
// expect: pass
func TestRunBuildWithNArchiveExporter(t *testing.T) {
	type testcase struct {
		descSpec string
		format   string
	}
	dockerfile := `
		FROM alpine:latest
		RUN echo hello world
		`
	wd, err := os.Getwd()
	assert.NilError(t, err)
	filePath := filepath.Join(wd, "Dockerfile")
	err = ioutil.WriteFile(filePath, []byte(dockerfile), constant.DefaultSharedFileMode)
	assert.NilError(t, err)
	defer func() {
		err = os.Remove(filePath)
		assert.NilError(t, err)
	}()

	mockBuild := newMockDaemon()
	ctx := context.Background()
	cli := newMockClient(&mockGrpcClient{imageBuildFunc: mockBuild.build})

	buildOpts.file = ""
	var args []string
	err = newBuildOptions(args)
	assert.NilError(t, err)

	var testcases = []testcase{
		{
			descSpec: "docker-daemon:isula:latest",
			format:   "docker",
		},
		{
			descSpec: "oci-archive:isula:latest",
			format:   "oci",
		},
	}
	for _, tc := range testcases {
		buildOpts.output = tc.descSpec
		buildOpts.format = tc.format
		gotImageID, err := runBuild(ctx, &cli)
		assert.NilError(t, err)
		assert.Equal(t, gotImageID, imageID)
	}
}

// Test runBuild
// case 1. docker-archive exporter
// expect: pass
func TestRunBuildWithArchiveExporter(t *testing.T) {
	type testcase struct {
		descSpec string
		format   string
	}
	dockerfile := `
		FROM alpine:latest
		RUN echo hello world
		`
	wd, err := os.Getwd()
	assert.NilError(t, err)
	filePath := filepath.Join(wd, "Dockerfile")
	err = ioutil.WriteFile(filePath, []byte(dockerfile), constant.DefaultSharedFileMode)
	assert.NilError(t, err)
	defer func() {
		err = os.Remove(filePath)
		assert.NilError(t, err)
	}()

	ctx := context.Background()
	cli := newMockClient(&mockGrpcClient{})

	buildOpts.file = ""
	var args []string
	err = newBuildOptions(args)
	assert.NilError(t, err)

	var testcases = []testcase{
		{
			descSpec: "docker-archive:/tmp/image:isula:latest",
			format:   "docker",
		},
		{
			descSpec: "oci-archive:/tmp/image:isula:latest",
			format:   "oci",
		},
	}
	for _, tc := range testcases {
		buildOpts.output = tc.descSpec
		buildOpts.format = tc.format
		gotImageID, err := runBuild(ctx, &cli)
		assert.NilError(t, err)
		assert.Equal(t, gotImageID, imageID)
	}
}

// Test readDockerfile
// case 1. file with full path
// expect: pass
func TestReadDockerfileWithFullpath(t *testing.T) {
	dockerfile := `
FROM alpine:latest
RUN echo hello world
`
	filename := "testDockerfile"
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile(filename, dockerfile))
	defer tmpDir.Remove()

	buildOpts.contextDir, _ = os.Getwd()
	buildOpts.file = tmpDir.Join(filename)

	_, _, err := readDockerfile()
	assert.NilError(t, err)
}

// Test readDockerfile
// case 2. file with only filename, will be resolved in contextDir
// expect: pass
func TestReadDockerfileWithFullname(t *testing.T) {
	dockerfile := `
FROM alpine:latest
RUN echo hello world
`
	filename := "testDockerfile"
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile(filename, dockerfile))
	defer tmpDir.Remove()

	buildOpts.contextDir = tmpDir.Path()
	buildOpts.file = "testDockerfile"

	_, _, err := readDockerfile()
	assert.NilError(t, err)
}

// Test readDockerfile
// case 3. file with no filename, will be resolved by contextDir+Dockerfile
// expect: pass
func TestReadDockerfileWithNoName(t *testing.T) {
	dockerfile := `
FROM alpine:latest
RUN echo hello world
`
	filename := "Dockerfile"
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile(filename, dockerfile))
	defer tmpDir.Remove()

	buildOpts.contextDir = tmpDir.Path()
	buildOpts.file = ""

	_, _, err := readDockerfile()
	assert.NilError(t, err)
}

// Test readDockerfile
// case 4. file with no content
// expect: return error
func TestReadDockerfileWithNoContent(t *testing.T) {
	dockerfile := ``

	filename := "Dockerfile"
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile(filename, dockerfile))
	defer tmpDir.Remove()

	buildOpts.contextDir = tmpDir.Path()
	buildOpts.file = filename

	_, _, err := readDockerfile()
	assert.ErrorContains(t, err, "file is empty")
}

// Test readDockerfile
// case 5. file with "directory"
// expect: return error
func TestReadDockerfileWithDirectory(t *testing.T) {
	buildOpts.contextDir = ""
	buildOpts.file = "."

	_, _, err := readDockerfile()
	assert.ErrorContains(t, err, "should be a regular file")
}

// Test readDockerfile
// case 6. buildOpts.file not appointed and contextDir has no file named Dockerfile and dockerfile
// expect: return error with dockerfile(If the Dockerfile is not existed, look for dockerfile)
func TestReadDockerfileWithNoNameAndNoFileNamedDockerfile(t *testing.T) {
	tmpDir := fs.NewDir(t, t.Name())
	defer tmpDir.Remove()

	buildOpts.contextDir = tmpDir.Path()
	buildOpts.file = ""

	_, _, err := readDockerfile()
	// if not found, os.Stat will tell us Dockerfile and dockerfile not found
	// so it depends on os.Stat's return
	assert.ErrorContains(t, err, "dockerfile: no such file or directory")
}

func TestNewBuildOptions(t *testing.T) {
	// no args case use current working directory as context directory
	cwd, err := os.Getwd()
	assert.NilError(t, err)
	realCwd, err := filepath.EvalSymlinks(cwd)
	assert.NilError(t, err)
	var args []string
	err = newBuildOptions(args)
	assert.NilError(t, err)
	assert.Equal(t, buildOpts.contextDir, realCwd)

	// normal case
	args = []string{".", "abc"}
	absPath, err := filepath.Abs(".")
	assert.NilError(t, err)
	realPath, err := filepath.EvalSymlinks(absPath)
	assert.NilError(t, err)
	err = newBuildOptions(args)
	assert.NilError(t, err)
	assert.Equal(t, buildOpts.contextDir, realPath)

	// context directory not exist
	tmpDir := fs.NewDir(t, t.Name())
	defer tmpDir.Remove()
	args = []string{tmpDir.Path() + "/test"}
	err = newBuildOptions(args)
	assert.ErrorContains(t, err, "error getting the real path")

	// context directory is not a directory
	err = ioutil.WriteFile(tmpDir.Path()+"/test", []byte(""), constant.DefaultRootFileMode)
	assert.NilError(t, err)
	args = []string{tmpDir.Path() + "/test"}
	err = newBuildOptions(args)
	assert.ErrorContains(t, err, "should be a directory")
}

func TestCheckAndProcessOut(t *testing.T) {
	type testcase struct {
		name   string
		output string
		errStr string
		isErr  bool
	}

	testcases := []testcase{
		{
			name:   "docker-archive",
			output: "docker-archive:/root/docker-archive.tar",
		},
		{
			name:   "docker-daemon",
			output: "docker-daemon:busybox:latest",
		},
		{
			name:   "docker-registry",
			output: "docker://registry.example.com/busybox:latest",
		},
		{
			name:   "empyty exporter",
			output: "",
		},
		{
			name:   "only has colon",
			output: ":",
			errStr: "transport should not be empty",
			isErr:  true,
		},
		{
			name:   "only has transport",
			output: "docker-archive:",
			errStr: "destination should not be empty",
			isErr:  true,
		},
		{
			name:   "invalid exporter with no dest1",
			output: "docker-archive",
			isErr:  true,
			errStr: "destination should not be empty",
		},
		{
			name:   "invalid exporter with no dest3",
			output: "docker-archive:  ",
			isErr:  true,
			errStr: "destination should not be empty",
		},
		{
			name:   "invalid exporter with no dest2",
			output: "docker-archive:",
			isErr:  true,
			errStr: "destination should not be empty",
		},
		{
			name:   "invalid exporter with no transport",
			output: ":/test/images",
			isErr:  true,
			errStr: "transport should not be empty",
		},
		{
			name:   "invalid transport",
			output: "docker-isula:/root/docker-isula.tar",
			errStr: "not support",
			isErr:  true,
		},
		{
			name:   "invalid docker transport longer than limit",
			output: "docker:lcoalhostaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:5000/isula:test",
			errStr: "output should not longer than",
			isErr:  true,
		},
		{
			name:   "invalid docker-daemon transport longer than limit",
			output: "docker-daemon:isulaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:test",
			errStr: "output should not longer than",
			isErr:  true,
		},
		{
			name:   "invalid docker-archive transport longer than limit",
			output: "docker-archive:isula.tar:isulaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:test",
			errStr: "output should not longer than",
			isErr:  true,
		},
		{
			name:   "invalid isulad transport longer than limit",
			output: "isulad:isulaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:test",
			errStr: "output should not longer than",
			isErr:  true,
		},
		{
			name:   "invalid isulad transport",
			output: "isulad:isula",
			errStr: "invalid isulad output format",
			isErr:  true,
		},
		{
			name:   "invalid isulad transport",
			output: "isulad:isula",
			errStr: "invalid isulad output format",
			isErr:  true,
		},
		{
			name:   "invalid isulad transport 2",
			output: "isulad:isula:isula:isula",
			errStr: "invalid isulad output format",
			isErr:  true,
		},
		{
			name:   "valid isulad transport",
			output: "isulad:isula:latest",
			isErr:  false,
		},
	}

	for _, tc := range testcases {
		buildOpts.buildID = "abc123"
		buildOpts.output = tc.output
		err := checkAndProcessOutput()
		if tc.isErr {
			assert.ErrorContains(t, err, tc.errStr, tc.name)
		} else {
			assert.NilError(t, err)
		}
	}
}

func TestEncryptBuildArgs(t *testing.T) {
	var tests = []struct {
		name    string
		args    []string
		encrypt bool
		err     bool
	}{
		{
			name:    "case 1 - no build-args",
			args:    []string{},
			encrypt: false,
			err:     false,
		},
		{
			name:    "case 2 - normal build-args",
			args:    []string{"foo=bar", "testArg=arg"},
			encrypt: false,
			err:     false,
		},
		{
			name:    "case 3 - sensitive build-args",
			args:    []string{"foo=bar", "http_proxy=http://username:password@url.com/"},
			encrypt: true,
			err:     false,
		},
	}

	tmpDir := fs.NewDir(t, t.Name())
	defer tmpDir.Remove()
	keyPath := filepath.Join(tmpDir.Path(), "isula-build.pub")
	privateKey, err := util.GenerateRSAKey(util.DefaultRSAKeySize)
	assert.NilError(t, err)
	err = util.GenRSAPublicKeyFile(privateKey, keyPath)
	assert.NilError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildOpts.buildArgs = tt.args
			if _, err := encryptBuildArgs(keyPath); (err == nil) != (!tt.err) {
				t.FailNow()
			}
			if tt.encrypt {
				for i := 0; i < len(tt.args); i++ {
					arg, err := util.DecryptRSA(buildOpts.buildArgs[i], privateKey, crypto.SHA512)
					assert.NilError(t, err)
					assert.Equal(t, tt.args[i], arg)
				}
			} else {
				for i := 0; i < len(tt.args); i++ {
					assert.Equal(t, tt.args[i], buildOpts.buildArgs[i])
				}
			}

		})
	}
}

func TestGetAbsPath(t *testing.T) {
	pwd, _ := os.Getwd()
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "TC1 - normal case with relative path",
			args:    args{path: "./imageID.txt"},
			want:    filepath.Join(pwd, "imageID.txt"),
			wantErr: false,
		},
		{
			name:    "TC2 - normal case with abs path",
			args:    args{path: filepath.Join(pwd, "imageID.txt")},
			want:    filepath.Join(pwd, "imageID.txt"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAbsPath(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAbsPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getAbsPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunBuildWithCap(t *testing.T) {
	tests := []struct {
		name    string
		caps    []string
		isErr   bool
		wantErr bool
	}{
		{
			name: "1 cap null",
			caps: []string{},
		},
		{
			name: "2 cap valid 1",
			caps: []string{"CAP_SYS_ADMIN"},
		},
		{
			name: "3 cap valid 2",
			caps: []string{"CAP_SYS_ADMIN", "CAP_CHOWN"},
		},
		{
			name:  "4 cap invalid 1",
			caps:  []string{"CAP_SYS_ADMINsss", "CAP_CHOWN"},
			isErr: true,
		},
		{
			name:  "5 cap invalid 2",
			caps:  []string{"CAP_SY2123", "CAP_CHOWN"},
			isErr: true,
		},
		{
			name:  "6 cap invalid 3",
			caps:  []string{"CAP_SYS_ADMINsss", "CAP_CHOWN123"},
			isErr: true,
		},
	}
	dockerfile := `
		FROM busybox:latest
		RUN echo hello world
		`
	tmpDir := fs.NewDir(t, t.Name(), fs.WithFile("Dockerfile", dockerfile))
	defer tmpDir.Remove()
	buildOpts.file = tmpDir.Join("Dockerfile")
	buildOpts.output = "docker-daemon:cap:latest"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildOpts.capAddList = tt.caps
			err := newBuildOptions([]string{tmpDir.Path()})
			if tt.isErr {
				assert.ErrorContains(t, err, "is invalid")
			}
			buildOpts.capAddList = nil
		})
	}
}
