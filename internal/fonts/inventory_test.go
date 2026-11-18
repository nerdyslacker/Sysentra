package fonts

import (
	"context"
	"reflect"
	"testing"

	"sysentra/internal/runner"
)

type stubRunner struct{ output string }

func (s stubRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{Stdout: []byte(s.output)}, nil
}

func TestInventoryListsUniqueSystemFontFamilies(t *testing.T) {
	inventory := NewInventory(stubRunner{output: "Noto Sans,Noto Sans UI\nDejaVu Sans\nNoto Sans\n\n"})
	got, err := inventory.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"DejaVu Sans", "Noto Sans", "Noto Sans UI"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fonts = %#v, want %#v", got, want)
	}
}
