package depsaccept

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type command struct {
	dir  string
	name string
	args []string
	env  []string
}

type runner interface {
	run(ctx context.Context, cmd command) error
	output(ctx context.Context, cmd command) ([]byte, error)
}

type realRunner struct {
	stdout io.Writer
	stderr io.Writer
}

type commandError struct {
	cmd    command
	err    error
	stderr string
}

func (e commandError) Error() string {
	text := strings.TrimSpace(e.stderr)
	if text == "" {
		return fmt.Sprintf("%s %s: %v", e.cmd.name, strings.Join(e.cmd.args, " "), e.err)
	}
	return fmt.Sprintf("%s %s: %v: %s", e.cmd.name, strings.Join(e.cmd.args, " "), e.err, text)
}

func (e commandError) Unwrap() error {
	return e.err
}

func (r realRunner) run(ctx context.Context, cmd command) error {
	process := exec.CommandContext(ctx, cmd.name, cmd.args...)
	process.Dir = cmd.dir
	process.Env = mergeEnv(cmd.env)
	process.Stdout = r.stdout
	process.Stderr = r.stderr
	if err := process.Run(); err != nil {
		return commandError{cmd: cmd, err: err}
	}
	return nil
}

func (r realRunner) output(ctx context.Context, cmd command) ([]byte, error) {
	process := exec.CommandContext(ctx, cmd.name, cmd.args...)
	process.Dir = cmd.dir
	process.Env = mergeEnv(cmd.env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	process.Stdout = &stdout
	process.Stderr = &stderr
	if err := process.Run(); err != nil {
		return nil, commandError{cmd: cmd, err: err, stderr: stderr.String()}
	}
	return stdout.Bytes(), nil
}

func mergeEnv(extra []string) []string {
	if len(extra) == 0 {
		return nil
	}
	merged := os.Environ()
	return append(merged, extra...)
}

func isExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func gitCommand(dir string, args ...string) command {
	return command{dir: dir, name: "git", args: args, env: []string{"GIT_MASTER=1"}}
}

func ghCommand(dir string, args ...string) command {
	return command{dir: dir, name: "gh", args: args}
}
