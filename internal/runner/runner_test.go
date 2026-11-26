package runner

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

func TestExecStreamsCompleteAndTrailingLines(t *testing.T) {
	var lines []string
	result, err := (Exec{}).RunStreaming(context.Background(), "/bin/sh", []string{"-c", "printf 'one\\ntwo'; printf 'error\\n' >&2"}, func(line string) {
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(lines)
	if !reflect.DeepEqual(lines, []string{"error", "one", "two"}) {
		t.Fatalf("lines = %#v", lines)
	}
	if string(result.Stdout) != "one\ntwo" || string(result.Stderr) != "error\n" {
		t.Fatalf("result = %#v", result)
	}
}
