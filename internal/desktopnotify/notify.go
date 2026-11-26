package desktopnotify

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

func Send(title, body string) error {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect notification bus: %w", err)
	}
	defer connection.Close()
	call := connection.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").Call(
		"org.freedesktop.Notifications.Notify", 0,
		"Sysentra", uint32(0), "io.github.nerdyslacker.Sysentra", title, body,
		[]string{}, map[string]dbus.Variant{}, int32(10_000),
	)
	if call.Err != nil {
		return fmt.Errorf("send desktop notification: %w", call.Err)
	}
	return nil
}
