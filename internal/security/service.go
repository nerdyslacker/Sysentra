package security

import (
	"errors"
	"regexp"
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
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]{0,127}$`)

type ServiceRequest struct {
	Version        int              `json:"version"`
	Operation      ServiceOperation `json:"operation"`
	Name           string           `json:"name"`
	TimeoutSeconds int              `json:"timeoutSeconds,omitempty"`
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
	switch r.Operation {
	case ServiceEnable, ServiceDisable, ServiceStart, ServiceStop, ServiceRestart, ServiceReload, ServiceOnce, ServiceKill, ServicePause, ServiceContinue:
		return nil
	default:
		return errors.New("unsupported service operation")
	}
}
