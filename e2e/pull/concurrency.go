// Copyright (c) 2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package pull

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
)

func (c ctx) testConcurrencyConfig(t *testing.T) {
	tests := []struct {
		name             string
		setting          string
		value            string
		expectedExitCode int
	}{
		{"DownloadConcurrency", "download concurrency", "5", 0},
		{"InvalidDownloadConcurrency", "download concurrency", "-1", 255},
		{"DownloadPartSize", "download part size", "32768", 0},
		{"InvalidDownloadPartSize", "download part size", "-1", 255},
		{"DownloadBufferSize", "download buffer size", "65536", 0},
		{"InvalidDownloadBufferSize", "download buffer size", "-1", 255},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name+"-set"),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("config global"),
			e2e.WithArgs("--set", tt.setting, tt.value),
			e2e.ExpectExit(tt.expectedExitCode),
		)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name+"-reset"),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("config global"),
			e2e.WithArgs("--reset", tt.setting),
			e2e.ExpectExit(0),
		)
	}
}

func (c ctx) testConcurrentPulls(t *testing.T) {
	const srcURI = "oras://ghcr.io/apptainer/alpine:3.15.0"

	tests := []struct {
		name             string
		settings         map[string]string
		envVars          []string
		expectedExitCode int
	}{
		// test traditional sequential download
		{"Concurrency1Cfg", map[string]string{"download concurrency": "1"}, nil, 0},
		// test concurrency 3
		{"Concurrency3Cfg", map[string]string{"download concurrency": "3"}, nil, 0},
		// test concurrency 10
		{"Concurrency10Cfg", map[string]string{"download concurrency": "10"}, nil, 0},

		// test 1/3/10 goroutines (set via env vars)
		{"Concurrency1Env", nil, []string{"APPTAINER_DOWNLOAD_CONCURRENCY=1"}, 0},
		{"Concurrency3Env", nil, []string{"APPTAINER_DOWNLOAD_CONCURRENCY=3"}, 0},
		{"Concurrency10Env", nil, []string{"APPTAINER_DOWNLOAD_CONCURRENCY=10"}, 0},

		// test concurrent download with 1 MiB and 8 MiB part size
		{"PartSize1MCfg", map[string]string{"download part size": "1048576"}, nil, 0},
		{"PartSize8MCfg", map[string]string{"download part size": "8388608"}, nil, 0},

		// test concurrent download with 1 MiB and 8 MiB part size (via env vars)
		{"PartSize1MEnv", nil, []string{"APPTAINER_DOWNLOAD_PART_SIZE=1048576"}, 0},
		{"PartSize8MEnv", nil, []string{"APPTAINER_DOWNLOAD_PART_SIZE=8388608"}, 0},

		// use 8 byte and 64 KiB buffer size for concurrent downloads
		{"BufferSize1Cfg", map[string]string{"download buffer size": "8"}, nil, 0},
		{"BufferSize65536Cfg", map[string]string{"download buffer size": "65536"}, nil, 0},

		// use 8 byte and 64 KiB buffer size for concurrent downloads (via env vars)
		{"BufferSize1Env", nil, []string{"APPTAINER_DOWNLOAD_BUFFER_SIZE=8"}, 0},
		{"BufferSize65536Env", nil, []string{"APPTAINER_DOWNLOAD_BUFFER_SIZE=65536"}, 0},

		// multiple settings (concurrency 1, download buffer size 64 KiB)
		{"MultipleSettings", map[string]string{"download concurrency": "1", "download buffer size": "65536"}, nil, 0},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp(c.env.TestDir, "pull_test.")
			if err != nil {
				t.Fatalf("Failed to create temporary directory for pull test: %+v", err)
			}
			defer os.RemoveAll(tmpdir)

			// Set global configuration
			if tt.settings != nil {
				cfgCmdOps := []e2e.ApptainerCmdOp{
					e2e.WithProfile(e2e.RootProfile),
					e2e.WithCommand("config global"),
					e2e.ExpectExit(0),
				}

				for key, value := range tt.settings {
					t.Logf("set %s %s", key, value)
					cfgCmd := append(cfgCmdOps, e2e.WithArgs("--set", key, value))
					c.env.RunApptainer(t, cfgCmd...)

					t.Cleanup(func() {
						t.Logf("reset %s", key)
						c.env.RunApptainer(
							t,
							e2e.WithProfile(e2e.RootProfile),
							e2e.WithCommand("config global"),
							e2e.WithArgs("--reset", key),
							e2e.ExpectExit(0),
						)
					})
				}
			}

			// Reset global configuration at test completion

			ts := testStruct{
				desc:             "",
				srcURI:           srcURI,
				expectedExitCode: tt.expectedExitCode,
				expectedImage:    getImageNameFromURI(srcURI),
				envVars:          tt.envVars,
			}

			// No explicit image path specified. Will use temp dir as working directory,
			// so we pull into a clean location.
			ts.workDir = tmpdir
			imageName := getImageNameFromURI(ts.srcURI)
			ts.expectedImage = filepath.Join(tmpdir, imageName)

			// if there's a pullDir, that's where we expect to find the image
			if ts.pullDir != "" {
				ts.expectedImage = filepath.Join(ts.pullDir, imageName)
			}

			// pull image
			c.imagePull(t, ts)
		})

	}
}
