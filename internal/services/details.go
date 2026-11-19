package services

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxServiceLogBytes = 64 * 1024

type Details struct {
	Definition string   `json:"definition"`
	RunScript  string   `json:"runScript"`
	LogPath    string   `json:"logPath"`
	Logs       []string `json:"logs"`
}

func serviceDetails(root, name string, system bool) (Details, error) {
	definition, err := serviceDirectory(root, name)
	if err != nil {
		return Details{}, err
	}
	details := Details{Definition: definition}
	if run, err := readBounded(filepath.Join(definition, "run"), 16*1024); err == nil {
		details.RunScript = strings.TrimSpace(string(run))
	}
	candidates := []string{filepath.Join(definition, "log", "main", "current"), filepath.Join(definition, "log", "current")}
	if system {
		candidates = append(candidates, filepath.Join("/var/log", name, "current"), filepath.Join("/var/log", name+".log"))
	}
	for _, path := range candidates {
		lines, err := readLogTail(path)
		if err == nil {
			details.LogPath, details.Logs = path, lines
			break
		}
	}
	if system && details.LogPath == "" {
		const socklog = "/var/log/socklog/everything/current"
		if lines, err := readLogTail(socklog); err == nil {
			for _, line := range lines {
				if strings.Contains(line, name) {
					details.Logs = append(details.Logs, line)
				}
			}
			if len(details.Logs) > 0 {
				details.LogPath = socklog
			}
		}
	}
	return details, nil
}

func serviceDirectory(root, name string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve service root: %w", err)
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return "", fmt.Errorf("resolve service definition: %w", err)
	}
	if filepath.Dir(path) != resolvedRoot {
		return "", errors.New("service definition escapes the allowed directory")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errors.New("service definition is not a directory")
	}
	return path, nil
}

func readLogTail(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("service log is not a regular file")
	}
	start := info.Size() - maxServiceLogBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, 0); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, maxServiceLogBytes))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if start > 0 && len(lines) > 1 {
		lines = lines[1:]
	}
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}
	return lines, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("file is not regular")
	}
	return io.ReadAll(io.LimitReader(file, limit))
}
