package runner

import (
	"context"
	"os/exec"
)

type Result struct {
	Stdout []byte
	Stderr []byte
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type Exec struct{}

func (Exec) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var result Result
	var err error
	result.Stdout, err = cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.Stderr = exitErr.Stderr
	}
	return result, err
}
