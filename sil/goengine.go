/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	goEngineOnce sync.Once
	goEngineErr  error
)

// GoEngineAvailable reports whether the go and wasm engines can build
// transpiled tests here: the go command is in PATH and royaljelly at
// RoyaljellyVersion resolves, from the module cache or the network. It is
// nil when they can, else the reason. Tests use it to skip those engines
// offline (GOPROXY=off, no network) instead of failing. The check runs once
// per process.
func GoEngineAvailable() error {
	goEngineOnce.Do(func() { goEngineErr = checkGoEngine() })
	return goEngineErr
}

func checkGoEngine() error {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return errors.New("go not found in PATH")
	}
	dir, err := os.MkdirTemp("", "beedance-goengine-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(dir+"/go.mod", []byte(goMod("", nil)), 0o600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, goTool, "mod", "download", "github.com/apiarytech/royaljelly")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("royaljelly %s does not resolve: %s", RoyaljellyVersion, strings.TrimSpace(string(out)))
	}
	return nil
}
