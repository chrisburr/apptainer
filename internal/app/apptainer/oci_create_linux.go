// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/oci"
	"github.com/apptainer/apptainer/internal/pkg/util/starter"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
)

// OciCreate creates a container from an OCI bundle
func OciCreate(containerID string, args *OciArgs) error {
	_, err := getState(containerID)
	if err == nil {
		return fmt.Errorf("%s already exists", containerID)
	}

	os.Clearenv()

	absBundle, err := filepath.Abs(args.BundlePath)
	if err != nil {
		return fmt.Errorf("failed to determine bundle absolute path: %s", err)
	}

	if err := os.Chdir(absBundle); err != nil {
		return fmt.Errorf("failed to change directory to %s: %s", absBundle, err)
	}

	engineConfig := oci.NewConfig()
	generator := generate.New(&engineConfig.OciConfig.Spec)
	engineConfig.SetBundlePath(absBundle)
	engineConfig.SetLogPath(args.LogPath)
	engineConfig.SetLogFormat(args.LogFormat)
	engineConfig.SetPidFile(args.PidFile)

	// load config.json from bundle path
	configJSON := filepath.Join(absBundle, "config.json")
	fb, err := os.Open(configJSON)
	if err != nil {
		return fmt.Errorf("oci specification file %q is missing or cannot be read", configJSON)
	}

	data, err := io.ReadAll(fb)
	if err != nil {
		return fmt.Errorf("failed to read OCI specification file %s: %s", configJSON, err)
	}

	fb.Close()

	if err := json.Unmarshal(data, generator.Config); err != nil {
		return fmt.Errorf("failed to parse OCI specification file %s: %s", configJSON, err)
	}

	engineConfig.EmptyProcess = args.EmptyProcess
	engineConfig.SyncSocket = args.SyncSocketPath

	commonConfig := &config.Common{
		ContainerID:  containerID,
		EngineName:   oci.Name,
		EngineConfig: engineConfig,
	}

	procName := fmt.Sprintf("Apptainer OCI %s", containerID)
	return starter.Run(
		procName,
		commonConfig,
		starter.WithStdin(os.Stdin),
		starter.WithStderr(os.Stderr),
		starter.WithStdout(os.Stdout),
	)
}
