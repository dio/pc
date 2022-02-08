// Copyright 2022 Dhi Aurrahman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
)

type Runner struct {
	ctx    context.Context
	cmd    *exec.Cmd
	binary string
}

// Run runs the prepared cmd, given an archive.
func (r *Runner) Run() error {
	err := r.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start %s: %w", r.binary, err)
	}

	// Wait until done.
	<-r.ctx.Done()

	if err = r.cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus, _ := exitError.Sys().(syscall.WaitStatus)
			if waitStatus.Signaled() {
				fmt.Println("process was signalled to shutdown")
			}
			return nil
		}
		return fmt.Errorf("failed to launch %s: %v", r.binary, err)
	}
	return nil
}

// New returns initialized command.
func New(ctx context.Context, binary string, args []string, out io.Writer) (*Runner, func(error)) {
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec
	cmd.Stdin = os.Stdin
	if out == nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// Allow to override the default stdout and stderr, for example by an io.MultiWriter().
		cmd.Stdout = out
		cmd.Stderr = out
	}

	return &Runner{
			ctx:    ctx,
			cmd:    cmd,
			binary: binary,
		}, func(error) {
			_ = cmd.Wait() // to make sure we are done.
		}
}
