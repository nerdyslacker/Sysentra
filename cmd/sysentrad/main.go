package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"

	"sysentra/internal/accounts"
	"sysentra/internal/daemon"
	"sysentra/internal/firewall"
	"sysentra/internal/firmware"
	"sysentra/internal/runner"
	"sysentra/internal/security"
	"sysentra/internal/services"
	"sysentra/internal/systemconfig"
	"sysentra/internal/transactions"
	"sysentra/internal/xbpsconfig"
)

const (
	busName       = "io.github.nerdyslacker.Sysentra.System1"
	objectPath    = dbus.ObjectPath("/io/github/nerdyslacker/Sysentra/System1")
	interfaceName = "io.github.nerdyslacker.Sysentra.System1"
)

const introspection = `<node>
  <interface name="io.github.nerdyslacker.Sysentra.System1">
    <method name="Version"><arg direction="out" type="u" name="version"/></method>
	<method name="AuthorizeSession"/>
    <method name="PlanPackage"><arg direction="in" type="s" name="request"/><arg direction="out" type="s" name="plan"/></method>
    <method name="CommitPackage"><arg direction="in" type="s" name="requestId"/><arg direction="out" type="s" name="result"/></method>
	<signal name="TransactionLog"><arg type="s" name="requestId"/><arg type="s" name="line"/></signal>
    <method name="ListServices"><arg direction="out" type="s" name="services"/></method>
    <method name="ServiceDetails"><arg direction="in" type="s" name="name"/><arg direction="out" type="s" name="details"/></method>
    <method name="ChangeService"><arg direction="in" type="s" name="request"/></method>
    <method name="ChangeXBPSRule"><arg direction="in" type="s" name="request"/></method>
    <method name="ChangeSystemConfig"><arg direction="in" type="s" name="request"/></method>
    <method name="ChangeAccount"><arg direction="in" type="s" name="request"/></method>
    <method name="AccountStatus"><arg direction="out" type="s" name="status"/></method>
    <method name="ChangeFirewall"><arg direction="in" type="s" name="request"/></method>
    <method name="ChangeFirmware"><arg direction="in" type="s" name="request"/><arg direction="out" type="s" name="result"/></method>
  </interface>` + introspect.IntrospectDataString + `</node>`

type api struct {
	connection   *dbus.Conn
	packages     *daemon.Service
	services     *daemon.ServiceController
	config       *daemon.XBPSConfigController
	systemConfig *daemon.SystemConfigController
	accounts     *daemon.AccountController
	firewall     *daemon.FirewallController
	firmware     *daemon.FirmwareController
}

func (api) Version() (uint32, *dbus.Error) { return security.HelperAPIVersion, nil }

func (a api) AuthorizeSession(sender dbus.Sender) *dbus.Error {
	if err := a.packages.Authorize(context.Background(), string(sender)); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (a api) PlanPackage(sender dbus.Sender, payload string) (string, *dbus.Error) {
	plan, err := a.packages.PlanPackageRequest(context.Background(), string(sender), payload)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(encoded), nil
}

func (a api) CommitPackage(sender dbus.Sender, requestID string) (string, *dbus.Error) {
	result, err := a.packages.CommitPackageStreaming(context.Background(), string(sender), requestID, func(line string) {
		a.emitTransactionLog(string(sender), requestID, line)
	})
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(encoded), nil
}

func (a api) emitTransactionLog(destination, requestID, line string) {
	message := &dbus.Message{
		Type: dbus.TypeSignal,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldInterface:   dbus.MakeVariant(interfaceName),
			dbus.FieldMember:      dbus.MakeVariant("TransactionLog"),
			dbus.FieldPath:        dbus.MakeVariant(objectPath),
			dbus.FieldDestination: dbus.MakeVariant(destination),
			dbus.FieldSignature:   dbus.MakeVariant(dbus.SignatureOf(requestID, line)),
		},
		Body: []any{requestID, line},
	}
	a.connection.Send(message, nil)
}

func (a api) ListServices() (string, *dbus.Error) {
	items, err := a.services.List(context.Background())
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(encoded), nil
}

func (a api) ServiceDetails(sender dbus.Sender, name string) (string, *dbus.Error) {
	details, err := a.services.Details(context.Background(), string(sender), name)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(encoded), nil
}

func (a api) ChangeService(sender dbus.Sender, payload string) *dbus.Error {
	if err := a.services.Change(context.Background(), string(sender), payload); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (a api) ChangeXBPSRule(sender dbus.Sender, payload string) *dbus.Error {
	if err := a.config.Change(context.Background(), string(sender), payload); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (a api) ChangeSystemConfig(sender dbus.Sender, payload string) *dbus.Error {
	if err := a.systemConfig.Change(context.Background(), string(sender), payload); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (a api) ChangeAccount(sender dbus.Sender, payload string) *dbus.Error {
	if err := a.accounts.Change(context.Background(), string(sender), payload); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (a api) AccountStatus(sender dbus.Sender) (string, *dbus.Error) {
	items, err := a.accounts.Status(context.Background(), string(sender))
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(encoded), nil
}

func (a api) ChangeFirewall(sender dbus.Sender, payload string) *dbus.Error {
	if err := a.firewall.Change(context.Background(), string(sender), payload); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

func (a api) ChangeFirmware(sender dbus.Sender, payload string) (string, *dbus.Error) {
	result, err := a.firmware.Change(context.Background(), string(sender), payload)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(encoded), nil
}

func main() {
	connection, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()
	commands := runner.Exec{}
	authorizer := security.NewPolkitAuthorizer(connection)
	packageService := daemon.NewService(authorizer, transactions.NewSystemXBPSPlanner(commands), transactions.NewSystemXBPSExecutor(commands), daemon.NewJSONAuditSink(os.Stdout))
	serviceInventory := services.NewInventory(commands, services.DefinitionsDir, services.EnabledDir, "/proc")
	serviceController := daemon.NewServiceController(authorizer, serviceInventory, services.NewManager(commands, services.DefinitionsDir, services.EnabledDir))
	configController := daemon.NewXBPSConfigController(authorizer, xbpsconfig.NewManager("/etc/xbps.d"))
	systemConfigController := daemon.NewSystemConfigController(authorizer, systemconfig.NewManager(commands, "/etc/xbps.d", "/usr/share/xbps.d", "/etc/samba/smb.conf", "/etc/fstab"))
	accountController := daemon.NewAccountController(authorizer, accounts.NewManager(commands))
	firewallInventory := firewall.NewInventory(commands)
	firewallController := daemon.NewFirewallController(authorizer, firewall.NewManager(commands, firewallInventory, services.NewManager(commands, services.DefinitionsDir, services.EnabledDir)))
	firmwareController := daemon.NewFirmwareController(authorizer, firmware.NewManager(commands))
	if err := connection.Export(api{connection: connection, packages: packageService, services: serviceController, config: configController, systemConfig: systemConfigController, accounts: accountController, firewall: firewallController, firmware: firmwareController}, objectPath, interfaceName); err != nil {
		log.Fatal(err)
	}
	if err := connection.Export(introspect.Introspectable(introspection), objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		log.Fatal(err)
	}
	reply, err := connection.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatal(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintln(os.Stderr, "sysentrad: D-Bus name is already owned")
		os.Exit(1)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}
