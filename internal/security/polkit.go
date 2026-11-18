package security

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

const PackageManagementAction = "io.github.nerdyslacker.sysentra.manage-packages"

var ErrNotAuthorized = errors.New("not authorized")

type PolkitAuthorizer struct {
	authority dbus.BusObject
}

type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}

type polkitResult struct {
	Authorized bool
	Challenge  bool
	Details    map[string]string
}

func NewPolkitAuthorizer(connection *dbus.Conn) *PolkitAuthorizer {
	return &PolkitAuthorizer{authority: connection.Object("org.freedesktop.PolicyKit1", "/org/freedesktop/PolicyKit1/Authority")}
}

func (a *PolkitAuthorizer) Authorize(ctx context.Context, sender string, action string, interactive bool) error {
	subject, err := polkitSubjectForSender(sender)
	if err != nil {
		return err
	}
	if !knownPolkitAction(action) {
		return fmt.Errorf("unknown authorization action %q", action)
	}
	flags := uint32(0)
	if interactive {
		flags = 1
	}
	var result polkitResult
	err = a.authority.CallWithContext(ctx, "org.freedesktop.PolicyKit1.Authority.CheckAuthorization", 0, subject, action, map[string]string{}, flags, "").Store(&result)
	if err != nil {
		return fmt.Errorf("check polkit authorization: %w", err)
	}
	if !result.Authorized {
		return ErrNotAuthorized
	}
	return nil
}

func knownPolkitAction(action string) bool {
	return action == PackageManagementAction || action == ServiceManagementAction || action == XBPSConfigManagementAction || action == SystemConfigManagementAction || action == AccountManagementAction
}

func polkitSubjectForSender(sender string) (polkitSubject, error) {
	if !strings.HasPrefix(sender, ":") || len(sender) < 4 || len(sender) > 255 {
		return polkitSubject{}, errors.New("invalid D-Bus sender")
	}
	return polkitSubject{Kind: "system-bus-name", Details: map[string]dbus.Variant{"name": dbus.MakeVariant(sender)}}, nil
}
