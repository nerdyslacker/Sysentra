//go:build !linux || !cgo

package flatpakplan

import "errors"

func resolve(_, _, _ string, _ []string) ([]Change, error) {
	return nil, errors.New("Flatpak transaction previews require Linux with cgo")
}
