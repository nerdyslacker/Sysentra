package runner

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
)

type Result struct {
	Stdout []byte
	Stderr []byte
}

type Runner interface {
	Run(context.Context, string, ...string) (Result, error)
}

type StreamingRunner interface {
	RunStreaming(context.Context, string, []string, func(string)) (Result, error)
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

func (Exec) RunStreaming(ctx context.Context, name string, args []string, output func(string)) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	sink := &lineSink{output: output}
	stdoutLines := &lineWriter{sink: sink}
	stderrLines := &lineWriter{sink: sink}
	cmd.Stdout = multiWriter{buffer: &stdout, lines: stdoutLines}
	cmd.Stderr = multiWriter{buffer: &stderr, lines: stderrLines}
	err := cmd.Run()
	stdoutLines.flush()
	stderrLines.flush()
	return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, err
}

type multiWriter struct {
	buffer *bytes.Buffer
	lines  *lineWriter
}

func (w multiWriter) Write(data []byte) (int, error) {
	w.buffer.Write(data)
	return w.lines.Write(data)
}

type lineWriter struct {
	mu     sync.Mutex
	buffer string
	sink   *lineSink
}

type lineSink struct {
	mu     sync.Mutex
	output func(string)
}

func (w *lineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer += string(data)
	for {
		index := strings.IndexByte(w.buffer, '\n')
		if index < 0 {
			break
		}
		w.emit(w.buffer[:index])
		w.buffer = w.buffer[index+1:]
	}
	return len(data), nil
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer != "" {
		w.emit(w.buffer)
		w.buffer = ""
	}
}

func (w *lineWriter) emit(line string) {
	if w.sink.output != nil {
		w.sink.mu.Lock()
		defer w.sink.mu.Unlock()
		w.sink.output(strings.TrimSuffix(line, "\r"))
	}
}
