package transactions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"

	"sysentra/internal/accounts"
	"sysentra/internal/firmware"
	"sysentra/internal/security"
	"sysentra/internal/services"
)

const (
	systemBusName    = "io.github.nerdyslacker.Sysentra.System1"
	systemObjectPath = dbus.ObjectPath("/io/github/nerdyslacker/Sysentra/System1")
	systemInterface  = "io.github.nerdyslacker.Sysentra.System1"
)

type DBusClient struct {
	connection *dbus.Conn
	err        error
}

func (c *DBusClient) checkVersion(ctx context.Context) error {
	if c.err != nil {
		return fmt.Errorf("connect to system service: %w", c.err)
	}
	var version uint32
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".Version", 0)
	if err := call.Store(&version); err != nil {
		return fmt.Errorf("query privileged helper version: %w", err)
	}
	if version != security.HelperAPIVersion {
		return fmt.Errorf("installed privileged helper is outdated (found version %d, need %d); reinstall bin/sysentrad", version, security.HelperAPIVersion)
	}
	return nil
}

func (c *DBusClient) AuthorizeSession(ctx context.Context) error {
	if err := c.checkVersion(ctx); err != nil {
		return err
	}
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".AuthorizeSession", 0)
	if call.Err != nil {
		return fmt.Errorf("authorize Sysentra session: %w", call.Err)
	}
	return nil
}

func (c *DBusClient) ListServices(ctx context.Context) ([]services.Service, error) {
	if c.err != nil {
		return nil, fmt.Errorf("connect to system service: %w", c.err)
	}
	var encoded string
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ListServices", 0)
	if err := call.Store(&encoded); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	var items []services.Service
	if err := json.Unmarshal([]byte(encoded), &items); err != nil {
		return nil, fmt.Errorf("decode service inventory: %w", err)
	}
	return items, nil
}

func (c *DBusClient) ServiceDetails(ctx context.Context, name string) (services.Details, error) {
	if err := c.checkVersion(ctx); err != nil {
		return services.Details{}, err
	}
	var encoded string
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ServiceDetails", 0, name)
	if err := call.Store(&encoded); err != nil {
		return services.Details{}, fmt.Errorf("read service details: %w", err)
	}
	var details services.Details
	if err := json.Unmarshal([]byte(encoded), &details); err != nil {
		return services.Details{}, fmt.Errorf("decode service details: %w", err)
	}
	return details, nil
}

func (c *DBusClient) ChangeService(ctx context.Context, request security.ServiceRequest) error {
	if err := c.checkVersion(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ChangeService", 0, string(payload))
	if call.Err != nil {
		return fmt.Errorf("change service: %w", call.Err)
	}
	return nil
}

func (c *DBusClient) ChangeXBPSRule(ctx context.Context, request security.XBPSRuleRequest) error {
	if c.err != nil {
		return fmt.Errorf("connect to system service: %w", c.err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ChangeXBPSRule", 0, string(payload))
	if call.Err != nil {
		return fmt.Errorf("change XBPS rule: %w", call.Err)
	}
	return nil
}

func (c *DBusClient) ChangeSystemConfig(ctx context.Context, request security.SystemConfigRequest) error {
	if c.err != nil {
		return fmt.Errorf("connect to system service: %w", c.err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ChangeSystemConfig", 0, string(payload))
	if call.Err != nil {
		return fmt.Errorf("change system configuration: %w", call.Err)
	}
	return nil
}

func (c *DBusClient) ChangeAccount(ctx context.Context, request security.AccountRequest) error {
	if err := c.checkVersion(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ChangeAccount", 0, string(payload))
	if call.Err != nil {
		return fmt.Errorf("change account: %w", call.Err)
	}
	return nil
}

func (c *DBusClient) AccountStatus(ctx context.Context) ([]accounts.Status, error) {
	if err := c.checkVersion(ctx); err != nil {
		return nil, err
	}
	var payload string
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".AccountStatus", 0)
	if call.Err != nil {
		return nil, fmt.Errorf("query account status: %w", call.Err)
	}
	if err := call.Store(&payload); err != nil {
		return nil, fmt.Errorf("decode account status response: %w", err)
	}
	var statuses []accounts.Status
	if err := json.Unmarshal([]byte(payload), &statuses); err != nil {
		return nil, fmt.Errorf("parse account status response: %w", err)
	}
	return statuses, nil
}

func (c *DBusClient) ChangeFirewall(ctx context.Context, request security.FirewallRequest) error {
	if err := c.checkVersion(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ChangeFirewall", 0, string(payload))
	if call.Err != nil {
		return fmt.Errorf("change firewall: %w", call.Err)
	}
	return nil
}

func (c *DBusClient) ChangeFirmware(ctx context.Context, request security.FirmwareRequest) (firmware.Result, error) {
	if err := c.checkVersion(ctx); err != nil {
		return firmware.Result{}, err
	}
	if err := request.Validate(); err != nil {
		return firmware.Result{}, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return firmware.Result{}, err
	}
	var response string
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".ChangeFirmware", 0, string(payload))
	if call.Err != nil {
		return firmware.Result{}, fmt.Errorf("change firmware: %w", call.Err)
	}
	if err := call.Store(&response); err != nil {
		return firmware.Result{}, fmt.Errorf("decode firmware response: %w", err)
	}
	var result firmware.Result
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return firmware.Result{}, fmt.Errorf("parse firmware response: %w", err)
	}
	return result, nil
}

func NewSystemDBusClient() *DBusClient {
	connection, err := dbus.ConnectSystemBus()
	return &DBusClient{connection: connection, err: err}
}

func (c *DBusClient) Close() error {
	if c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

func (c *DBusClient) Plan(ctx context.Context, request security.PackageRequest) (Plan, error) {
	if err := c.checkVersion(ctx); err != nil {
		return Plan{}, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return Plan{}, err
	}
	var encoded string
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".PlanPackage", 0, string(payload))
	if err := call.Store(&encoded); err != nil {
		return Plan{}, fmt.Errorf("plan package transaction: %w", err)
	}
	var plan Plan
	if err := json.Unmarshal([]byte(encoded), &plan); err != nil {
		return Plan{}, fmt.Errorf("decode package transaction plan: %w", err)
	}
	return plan, nil
}

func (c *DBusClient) Commit(ctx context.Context, requestID string) (Result, error) {
	if c.err != nil {
		return Result{}, fmt.Errorf("connect to system service: %w", c.err)
	}
	var encoded string
	call := c.connection.Object(systemBusName, systemObjectPath).CallWithContext(ctx, systemInterface+".CommitPackage", 0, requestID)
	if err := call.Store(&encoded); err != nil {
		return Result{}, fmt.Errorf("commit package transaction: %w", err)
	}
	var result Result
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		return Result{}, fmt.Errorf("decode package transaction result: %w", err)
	}
	return result, nil
}
