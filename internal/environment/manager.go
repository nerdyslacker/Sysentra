package environment

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	blockStart = "# >>> Sysentra environment >>>"
	blockEnd   = "# <<< Sysentra environment <<<"
)

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

type Variable struct {
	Name, Value, Scope, File string
}

type Manager struct{ home string }

func NewManager(home string) *Manager { return &Manager{home: home} }

func ListGlobal(path string) ([]Variable, error) {
	contents, err := readRegular(path)
	if err != nil {
		return nil, err
	}
	var variables []Variable
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok || !namePattern.MatchString(name) {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		variables = append(variables, Variable{Name: name, Value: value, Scope: "Global", File: path})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i].Name < variables[j].Name })
	return variables, nil
}

func Validate(name, value string) error {
	if !namePattern.MatchString(name) {
		return errors.New("environment variable name must contain only letters, digits, and underscores and cannot start with a digit")
	}
	if len(value) > 4096 || strings.ContainsAny(value, "\x00\r\n") {
		return errors.New("environment variable value must be a single line no longer than 4096 bytes")
	}
	return nil
}

func (m *Manager) List() ([]Variable, error) {
	var variables []Variable
	for _, scope := range []string{"bash", "zsh", "fish"} {
		for _, path := range m.paths(scope) {
			contents, err := readRegular(path)
			if err != nil {
				return nil, err
			}
			scanner := bufio.NewScanner(strings.NewReader(string(contents)))
			for scanner.Scan() {
				name, value, ok := parseLine(strings.TrimSpace(scanner.Text()), scope)
				if ok {
					variables = append(variables, Variable{Name: name, Value: value, Scope: title(scope), File: path})
				}
			}
			if err := scanner.Err(); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(variables, func(i, j int) bool {
		if variables[i].Scope == variables[j].Scope {
			return variables[i].Name < variables[j].Name
		}
		return variables[i].Scope < variables[j].Scope
	})
	return variables, nil
}

func (m *Manager) Change(scope, operation, name, oldName, value string, source ...string) error {
	if scope != "bash" && scope != "zsh" && scope != "fish" {
		return errors.New("unsupported shell")
	}
	if err := Validate(name, value); err != nil && operation != "remove" {
		return err
	}
	if operation == "remove" {
		if !namePattern.MatchString(name) {
			return errors.New("invalid environment variable name")
		}
	} else if operation != "set" {
		return errors.New("unsupported environment variable operation")
	}
	if oldName != "" && !namePattern.MatchString(oldName) {
		return errors.New("invalid previous environment variable name")
	}
	path := m.path(scope)
	if len(source) > 0 && source[0] != "" {
		path = filepath.Clean(source[0])
		if !slices.Contains(m.paths(scope), path) {
			return errors.New("shell configuration file is outside the supported allowlist")
		}
	}
	contents, err := readRegular(path)
	if err != nil {
		return err
	}
	target := name
	if oldName != "" {
		target = oldName
	}
	if len(source) > 0 && source[0] != "" {
		changed, found := changeSourceLine(contents, scope, operation, target, name, value)
		if !found {
			return errors.New("environment variable was not found in its source file")
		}
		return writeRegular(path, changed)
	}
	variables := managedVariables(contents, scope)
	if operation == "remove" {
		if _, exists := variables[target]; !exists {
			return errors.New("managed environment variable was not found")
		}
		delete(variables, target)
		return writeRegular(path, replaceBlock(contents, scope, variables))
	}
	if oldName != "" && oldName != name {
		delete(variables, oldName)
	}
	variables[name] = value
	return writeRegular(path, replaceBlock(contents, scope, variables))
}

func (m *Manager) path(scope string) string {
	if scope == "fish" {
		return filepath.Join(m.home, ".config", "fish", "config.fish")
	}
	return filepath.Join(m.home, "."+scope+"rc")
}

func (m *Manager) paths(scope string) []string {
	switch scope {
	case "bash":
		return []string{filepath.Join(m.home, ".bashrc"), filepath.Join(m.home, ".bash_profile"), filepath.Join(m.home, ".bash_login"), filepath.Join(m.home, ".profile")}
	case "zsh":
		return []string{filepath.Join(m.home, ".zshrc"), filepath.Join(m.home, ".zprofile"), filepath.Join(m.home, ".zshenv"), filepath.Join(m.home, ".zlogin")}
	case "fish":
		paths := []string{filepath.Join(m.home, ".config", "fish", "config.fish")}
		files, _ := filepath.Glob(filepath.Join(m.home, ".config", "fish", "conf.d", "*.fish"))
		return append(paths, files...)
	default:
		return nil
	}
}

func managedVariables(contents []byte, scope string) map[string]string {
	variables := map[string]string{}
	inBlock := false
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == blockStart {
			inBlock = true
			continue
		}
		if line == blockEnd {
			break
		}
		if !inBlock {
			continue
		}
		name, value, ok := parseLine(line, scope)
		if ok {
			variables[name] = value
		}
	}
	return variables
}

func parseLine(line, scope string) (string, string, bool) {
	prefix := "export "
	if scope == "fish" {
		prefix = "set -gx "
	}
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	separator := "="
	if scope == "fish" {
		separator = " "
	}
	name, quoted, ok := strings.Cut(rest, separator)
	if !ok || !namePattern.MatchString(name) {
		return "", "", false
	}
	value := strings.TrimSpace(quoted)
	if unquoted, ok := unquoteShell(value, scope); ok {
		value = unquoted
	}
	return name, value, true
}

func changeSourceLine(contents []byte, scope, operation, target, name, value string) ([]byte, bool) {
	lines := strings.Split(strings.TrimRight(string(contents), "\n"), "\n")
	for index, line := range lines {
		found, _, ok := parseLine(strings.TrimSpace(line), scope)
		if !ok || found != target {
			continue
		}
		if operation == "remove" {
			lines = append(lines[:index], lines[index+1:]...)
		} else if scope == "fish" {
			lines[index] = "set -gx " + name + " " + quoteShell(value, scope)
		} else {
			lines[index] = "export " + name + "=" + quoteShell(value, scope)
		}
		return []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n") + trailingNewline(lines)), true
	}
	return contents, false
}

func trailingNewline(lines []string) string {
	if len(lines) == 0 || len(lines) == 1 && lines[0] == "" {
		return ""
	}
	return "\n"
}

func replaceBlock(contents []byte, scope string, variables map[string]string) []byte {
	lines := strings.Split(strings.TrimRight(string(contents), "\n"), "\n")
	start, end := -1, -1
	for index, line := range lines {
		if strings.TrimSpace(line) == blockStart {
			start = index
		}
		if start >= 0 && strings.TrimSpace(line) == blockEnd {
			end = index + 1
			break
		}
	}
	if start >= 0 {
		if end < 0 {
			end = len(lines)
		}
		lines = append(lines[:start], lines[end:]...)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(variables) > 0 {
		names := make([]string, 0, len(variables))
		for name := range variables {
			names = append(names, name)
		}
		sort.Strings(names)
		block := []string{blockStart}
		for _, name := range names {
			assignment := "export " + name + "=" + quoteShell(variables[name], scope)
			if scope == "fish" {
				assignment = "set -gx " + name + " " + quoteShell(variables[name], scope)
			}
			block = append(block, assignment)
		}
		block = append(block, blockEnd)
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func quoteShell(value, scope string) string {
	if scope == "fish" {
		return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value) + "'"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func unquoteShell(value, scope string) (string, bool) {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		unquoted, err := strconv.Unquote(value)
		return unquoted, err == nil
	}
	if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
		return "", false
	}
	value = value[1 : len(value)-1]
	if scope == "fish" {
		return strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(value), true
	}
	return strings.ReplaceAll(value, `'\''`, "'"), true
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

func writeRegular(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sysentra-env-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func title(value string) string { return strings.ToUpper(value[:1]) + value[1:] }
