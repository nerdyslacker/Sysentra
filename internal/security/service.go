package security

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

const ServiceManagementAction = "io.github.nerdyslacker.sysentra.manage-services"

type ServiceOperation string

const (
	ServiceEnable   ServiceOperation = "enable"
	ServiceDisable  ServiceOperation = "disable"
	ServiceStart    ServiceOperation = "start"
	ServiceStop     ServiceOperation = "stop"
	ServiceRestart  ServiceOperation = "restart"
	ServiceReload   ServiceOperation = "reload"
	ServiceOnce     ServiceOperation = "once"
	ServiceKill     ServiceOperation = "kill"
	ServicePause    ServiceOperation = "pause"
	ServiceContinue ServiceOperation = "cont"
	ServiceInitUser ServiceOperation = "init-user"
	ServiceAdd      ServiceOperation = "add"
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]{0,127}$`)

type ServiceRequest struct {
	Version        int              `json:"version"`
	Operation      ServiceOperation `json:"operation"`
	Name           string           `json:"name"`
	TimeoutSeconds int              `json:"timeoutSeconds,omitempty"`
	Command        string           `json:"command,omitempty"`
	Arguments      []string         `json:"arguments,omitempty"`
}

func (r ServiceRequest) Validate() error {
	if r.Version != APIVersion {
		return errors.New("unsupported API version")
	}
	if !serviceNamePattern.MatchString(r.Name) {
		return errors.New("invalid service name")
	}
	if r.TimeoutSeconds < 0 || r.TimeoutSeconds > 60 {
		return errors.New("service timeout must be between 1 and 60 seconds")
	}
	if r.Operation == ServiceAdd {
		if !filepath.IsAbs(r.Command) || filepath.Clean(r.Command) != r.Command || len(r.Command) > 4096 || strings.ContainsRune(r.Command, 0) {
			return errors.New("service command must be a clean absolute path")
		}
		if len(r.Arguments) > 64 {
			return errors.New("service command accepts at most 64 arguments")
		}
		for _, argument := range r.Arguments {
			if len(argument) > 4096 || strings.ContainsRune(argument, 0) {
				return errors.New("invalid service command argument")
			}
		}
		return nil
	}
	if r.Command != "" || len(r.Arguments) != 0 {
		return errors.New("service command is only valid when adding a service")
	}
	switch r.Operation {
	case ServiceEnable, ServiceDisable, ServiceStart, ServiceStop, ServiceRestart, ServiceReload, ServiceOnce, ServiceKill, ServicePause, ServiceContinue, ServiceInitUser:
		return nil
	default:
		return errors.New("unsupported service operation")
	}
}
