// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package gomodcmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Runner allows to run certain commands against module aware Go CLI.
type Runner struct {
	goCmd    string
	modDir   string
	insecure bool

	verbose bool
}

// NewRunner checks Go version compatibility and initialize new go.mod in the modDir if not yet present, then returns Runner.
func NewRunner(ctx context.Context, insecure bool, modDir string, goCmd string) (*Runner, error) {
	r := &Runner{
		goCmd:    goCmd,
		modDir:   modDir,
		insecure: insecure,
	}

	ver, err := r.execGo(ctx, "version")
	if err != nil {
		return nil, errors.Wrap(err, "exec go to detect the version")
	}

	if !strings.HasPrefix(ver, "go version go1.14.") {
		return nil, errors.Errorf("found unsupported go version: %v. Requires go1.14.x", ver)
	}

	if err := os.MkdirAll(modDir, os.ModePerm); err != nil {
		return nil, errors.Wrapf(err, "create moddir %s", modDir)
	}

	if _, err := os.Stat(filepath.Join(r.modDir, "go.mod")); err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "stat module file %s", filepath.Join(r.modDir, "go.mod"))
		}
		currMod, err := r.execGo(ctx, "list", "-m")
		if err != nil {
			return nil, err
		}

		// TODO(bwplotka): Check if currMod is not gobin..

		if _, err := r.execGoInModDir(ctx, "mod", "init", filepath.Join(currMod, r.modDir)); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (c *Runner) execGo(ctx context.Context, args ...string) (string, error) {
	return c.exec(ctx, "", c.goCmd, args...)
}

func (c *Runner) execGoInModDir(ctx context.Context, args ...string) (string, error) {
	return c.exec(ctx, c.modDir, c.goCmd, args...)
}

func (c *Runner) exec(ctx context.Context, cd string, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = filepath.Join(cmd.Dir, cd)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			if c.verbose {
				return "", errors.Errorf("error while running command '%s %s'; out: %s; err: %v", command, strings.Join(args, " "), b.String(), err)
			}
			return "", errors.New(b.String())

		}
		return "", errors.Errorf("error while running command '%s %s'; out: %s; err: %v", command, strings.Join(args, " "), b.String(), err)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

type GetUpdatePolicy string

const (
	NoUpdatePolicy    = GetUpdatePolicy("")
	UpdatePolicy      = GetUpdatePolicy("-u")
	UpdatePatchPolicy = GetUpdatePolicy("-u=patch")
)

// GetD runs 'go get -d' against separate go modules file with given arguments.
func (c *Runner) GetD(ctx context.Context, update GetUpdatePolicy, packages ...string) error {
	args := []string{"get", "-d"}
	if c.insecure {
		args = append(args, "-insecure")
	}

	if update != NoUpdatePolicy {
		args = append(args, string(update))
	}
	_, err := c.execGoInModDir(ctx, append(args, packages...)...)
	return err
}

// Installs runs 'go install' against separate go modules file with given packages.
func (c *Runner) Install(ctx context.Context, packages ...string) error {
	_, err := c.execGoInModDir(ctx, append([]string{"install"}, packages...)...)
	return err
}

// ModTidy runs 'go mod tidy' against separate go modules file.
func (c *Runner) ModTidy(ctx context.Context) error {
	_, err := c.execGoInModDir(ctx, "mod", "tidy")
	return err
}