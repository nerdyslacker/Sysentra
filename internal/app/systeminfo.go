package app

import (
	"os"
	"strconv"
	"strings"
)

type SystemInfoView struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (*SystemService) GetSystemInfo() (SystemInfoView, error) {
	contents, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return SystemInfoView{}, err
	}
	return parseOSRelease(contents), nil
}

func parseOSRelease(contents []byte) SystemInfoView {
	values := make(map[string]string)
	for _, line := range strings.Split(string(contents), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		values[key] = value
	}
	name := values["PRETTY_NAME"]
	if name == "" {
		name = values["NAME"]
	}
	if name == "" {
		name = values["ID"]
	}
	if name == "" {
		name = "Linux"
	}
	return SystemInfoView{Name: name, Version: values["VERSION_ID"]}
}
