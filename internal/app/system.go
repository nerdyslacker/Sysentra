package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sysentra/internal/accounts"
	"sysentra/internal/drivers"
	"sysentra/internal/firewall"
	"sysentra/internal/firmware"
	"sysentra/internal/kernel"
	"sysentra/internal/mirrors"
	"sysentra/internal/repositories"
	"sysentra/internal/security"
	"sysentra/internal/services"
	"sysentra/internal/settings"
	"sysentra/internal/shares"
	"sysentra/internal/transactions"
	"sysentra/internal/xbpsconfig"
)

type kernelReader interface {
	List(context.Context) ([]kernel.Kernel, error)
	Status(context.Context) (kernel.Status, error)
}
type repositoryReader interface {
	List(context.Context) ([]repositories.Repository, error)
}
type mirrorReader interface {
	List(context.Context) (mirrors.Catalog, error)
}
type mirrorRecoveryStore interface {
	Read() (settings.MirrorRecovery, error)
	Write(settings.MirrorRecovery) error
}
type xbpsConfigReader interface {
	List() ([]xbpsconfig.Rule, error)
}
type firmwareReader interface {
	List(context.Context) (firmware.Catalog, error)
}
type firewallReader interface {
	ReadBackend(context.Context, string) (firewall.State, error)
}
type driverReader interface {
	List(context.Context) (drivers.Catalog, error)
}
type serviceReader interface {
	ListServices(context.Context) ([]services.Service, error)
	ServiceDetails(context.Context, string) (services.Details, error)
	Plan(context.Context, security.PackageRequest) (transactions.Plan, error)
	Commit(context.Context, string) (transactions.Result, error)
	ChangeService(context.Context, security.ServiceRequest) error
	ChangeXBPSRule(context.Context, security.XBPSRuleRequest) error
	ChangeSystemConfig(context.Context, security.SystemConfigRequest) error
	ChangeAccount(context.Context, security.AccountRequest) error
	AccountStatus(context.Context) ([]accounts.Status, error)
	ChangeFirewall(context.Context, security.FirewallRequest) error
	ChangeFirmware(context.Context, security.FirmwareRequest) (firmware.Result, error)
}
type userServiceReader interface {
	List(context.Context) ([]services.Service, error)
	Details(string) (services.Details, error)
}
type userServiceManager interface {
	Change(context.Context, security.ServiceRequest) error
}
type shareReader interface {
	Samba() (shares.SambaServer, error)
	NFS() ([]shares.NFSMount, error)
}
type accountReader interface {
	List() ([]accounts.User, []accounts.Group, error)
}

type ServiceView struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Status     string `json:"status"`
	NextAction string `json:"nextAction"`
	PID        int    `json:"pid"`
	Command    string `json:"command"`
	Time       string `json:"time"`
}

type ServiceDetailsView struct {
	Definition string   `json:"definition"`
	RunScript  string   `json:"runScript"`
	LogPath    string   `json:"logPath"`
	Logs       []string `json:"logs"`
	Managed    bool     `json:"managed"`
}

type SambaShareView struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Comment        string `json:"comment"`
	ValidUsers     string `json:"validUsers"`
	WriteList      string `json:"writeList"`
	ReadList       string `json:"readList"`
	ForceUser      string `json:"forceUser"`
	ForceGroup     string `json:"forceGroup"`
	CreateMask     string `json:"createMask"`
	DirectoryMask  string `json:"directoryMask"`
	ReadOnly       bool   `json:"readOnly"`
	GuestOK        bool   `json:"guestOK"`
	Browseable     bool   `json:"browseable"`
	FollowSymlinks bool   `json:"followSymlinks"`
	WideLinks      bool   `json:"wideLinks"`
}
type SambaServerView struct {
	Configured bool             `json:"configured"`
	Shares     []SambaShareView `json:"shares"`
	Services   []ServiceView    `json:"services"`
	Global     SambaGlobalView  `json:"global"`
}
type SambaGlobalView struct {
	ConfigPath      string `json:"configPath"`
	Protocol        string `json:"protocol"`
	DefaultMask     string `json:"defaultMask"`
	Printing        bool   `json:"printing"`
	MapGuest        bool   `json:"mapGuest"`
	FirewallBackend string `json:"firewallBackend"`
	FirewallActive  bool   `json:"firewallActive"`
	FirewallAllowed bool   `json:"firewallAllowed"`
}
type SambaGlobalInput struct {
	Protocol        string `json:"protocol"`
	DefaultMask     string `json:"defaultMask"`
	Printing        bool   `json:"printing"`
	MapGuest        bool   `json:"mapGuest"`
	FirewallAllowed bool   `json:"firewallAllowed"`
}
type SambaShareInput struct {
	Operation      string `json:"operation"`
	Name           string `json:"name"`
	OldName        string `json:"oldName"`
	Path           string `json:"path"`
	Comment        string `json:"comment"`
	ValidUsers     string `json:"validUsers"`
	WriteList      string `json:"writeList"`
	ReadList       string `json:"readList"`
	ForceUser      string `json:"forceUser"`
	ForceGroup     string `json:"forceGroup"`
	CreateMask     string `json:"createMask"`
	DirectoryMask  string `json:"directoryMask"`
	ReadOnly       bool   `json:"readOnly"`
	GuestOK        bool   `json:"guestOK"`
	Browseable     bool   `json:"browseable"`
	FollowSymlinks bool   `json:"followSymlinks"`
	WideLinks      bool   `json:"wideLinks"`
}
type NFSMountView struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Type       string `json:"type"`
	Options    string `json:"options"`
	Configured bool   `json:"configured"`
	Mounted    bool   `json:"mounted"`
}

type UserView struct {
	Name         string   `json:"name"`
	UID          int      `json:"uid"`
	GID          int      `json:"gid"`
	Home         string   `json:"home"`
	Shell        string   `json:"shell"`
	Groups       []string `json:"groups"`
	Removable    bool     `json:"removable"`
	PrimaryGroup string   `json:"primaryGroup"`
	Password     string   `json:"password"`
	Processes    int      `json:"processes"`
	Sessions     int      `json:"sessions"`
}
type GroupView struct {
	Name      string   `json:"name"`
	GID       int      `json:"gid"`
	Members   []string `json:"members"`
	Removable bool     `json:"removable"`
}
type AccountsView struct {
	Users  []UserView  `json:"users"`
	Groups []GroupView `json:"groups"`
}

type AccountHistoryView struct {
	Time      string `json:"time"`
	Operation string `json:"operation"`
	Target    string `json:"target"`
}

type KernelView struct {
	Package     string `json:"package"`
	Version     string `json:"version"`
	Series      string `json:"series"`
	Type        string `json:"type"`
	Size        string `json:"size"`
	InstallDate string `json:"installDate"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	Meta        bool   `json:"meta"`
	Headers     bool   `json:"headers"`
	CanInstall  bool   `json:"canInstall"`
	CanRemove   bool   `json:"canRemove"`
}

type KernelMountView struct {
	Target     string `json:"target"`
	Source     string `json:"source"`
	FileSystem string `json:"fileSystem"`
	Size       uint64 `json:"size"`
	Available  uint64 `json:"available"`
	Mounted    bool   `json:"mounted"`
	Separate   bool   `json:"separate"`
}

type KernelBootImageView struct {
	Version  string `json:"version"`
	Status   string `json:"status"`
	Size     int64  `json:"size"`
	CanPurge bool   `json:"canPurge"`
}

type KernelDKMSView struct {
	Module    string `json:"module"`
	Version   string `json:"version"`
	Kernel    string `json:"kernel"`
	Arch      string `json:"arch"`
	Status    string `json:"status"`
	Package   bool   `json:"package"`
	Installed bool   `json:"installed"`
}

type KernelStatusView struct {
	BootMode        string                `json:"bootMode"`
	Bootloader      string                `json:"bootloader"`
	Boot            KernelMountView       `json:"boot"`
	EFI             KernelMountView       `json:"efi"`
	InitramfsTool   string                `json:"initramfsTool"`
	Images          []KernelBootImageView `json:"images"`
	DKMS            []KernelDKMSView      `json:"dkms"`
	SwitchSupported bool                  `json:"switchSupported"`
	SwitchReason    string                `json:"switchReason"`
	Warnings        []string              `json:"warnings"`
}

type RepositoryView struct {
	URL      string `json:"url"`
	Priority int    `json:"priority"`
	Origin   string `json:"origin"`
	Enabled  bool   `json:"enabled"`
	Mutable  bool   `json:"mutable"`
}

type MirrorView struct {
	URL        string `json:"url"`
	Region     string `json:"region"`
	Location   string `json:"location"`
	Tier       int    `json:"tier"`
	Status     string `json:"status"`
	LastSynced string `json:"lastSynced"`
	Current    bool   `json:"current"`
}

type MirrorCatalogView struct {
	Mirrors  []MirrorView `json:"mirrors"`
	Warnings []string     `json:"warnings"`
}

type MirrorRecoveryView struct {
	Enabled bool `json:"enabled"`
}

type XBPSConfigRuleView struct {
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Origin  string `json:"origin"`
	Mutable bool   `json:"mutable"`
}

type FirmwareDeviceView struct {
	Name             string `json:"name"`
	Vendor           string `json:"vendor"`
	Version          string `json:"version"`
	ID               string `json:"id"`
	Summary          string `json:"summary"`
	Updatable        bool   `json:"updatable"`
	NewVersion       string `json:"newVersion"`
	RebootRequired   bool   `json:"rebootRequired"`
	ShutdownRequired bool   `json:"shutdownRequired"`
	ACRequired       bool   `json:"acRequired"`
	UpdateRemote     string `json:"updateRemote"`
	UpdateSummary    string `json:"updateSummary"`
	UpdateMessage    string `json:"updateMessage"`
	UpdateUrgency    string `json:"updateUrgency"`
	UpdateSize       uint64 `json:"updateSize"`
}

type FirmwareCatalogView struct {
	Available bool                 `json:"available"`
	Devices   []FirmwareDeviceView `json:"devices"`
	Warning   string               `json:"warning"`
}

type FirmwareActionResultView struct {
	Catalog     FirmwareCatalogView `json:"catalog"`
	Diagnostics []string            `json:"diagnostics"`
}

type FirewallRuleView struct {
	Kind      string `json:"kind"`
	Target    string `json:"target"`
	Protocol  string `json:"protocol"`
	Direction string `json:"direction"`
	Action    string `json:"action"`
	Source    string `json:"source"`
	Mutable   bool   `json:"mutable"`
}

type FirewallView struct {
	Backend   string             `json:"backend"`
	Installed bool               `json:"installed"`
	Active    bool               `json:"active"`
	Unknown   bool               `json:"unknown"`
	Policy    string             `json:"policy"`
	Zone      string             `json:"zone"`
	Rules     []FirewallRuleView `json:"rules"`
	Backends  []string           `json:"backends"`
}

type FirewallChangeInput struct {
	Operation string           `json:"operation"`
	Backend   string           `json:"backend"`
	Zone      string           `json:"zone"`
	Policy    string           `json:"policy"`
	Direction string           `json:"direction"`
	Rule      FirewallRuleView `json:"rule"`
	OldRule   FirewallRuleView `json:"oldRule"`
}

type DriverDeviceView struct {
	Category string `json:"category"`
	Bus      string `json:"bus"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type DriverCatalogView struct {
	Devices  []DriverDeviceView `json:"devices"`
	Warnings []string           `json:"warnings"`
}

type SystemService struct {
	kernels            kernelReader
	repositories       repositoryReader
	mirrors            mirrorReader
	recovery           mirrorRecoveryStore
	xbpsConfig         xbpsConfigReader
	firmware           firmwareReader
	firewall           firewallReader
	drivers            driverReader
	services           serviceReader
	userServices       userServiceReader
	userManager        userServiceManager
	turnstileServices  userServiceReader
	turnstileManager   *services.TurnstileManager
	shares             shareReader
	accounts           accountReader
	accountHistoryMu   sync.Mutex
	accountHistoryPath string
}

func NewSystemService(kernels kernelReader, repos repositoryReader, mirrorCatalog mirrorReader, recovery mirrorRecoveryStore, config xbpsConfigReader, firmwareInventory firmwareReader, firewallInventory firewallReader, driverInventory driverReader, serviceInventory serviceReader, userInventory userServiceReader, userManager userServiceManager, turnstileInventory userServiceReader, turnstileManager *services.TurnstileManager, shareInventory shareReader, accountInventory accountReader) *SystemService {
	service := &SystemService{kernels: kernels, repositories: repos, mirrors: mirrorCatalog, recovery: recovery, xbpsConfig: config, firmware: firmwareInventory, firewall: firewallInventory, drivers: driverInventory, services: serviceInventory, userServices: userInventory, userManager: userManager, turnstileServices: turnstileInventory, turnstileManager: turnstileManager, shares: shareInventory, accounts: accountInventory}
	if configDir, err := os.UserConfigDir(); err == nil {
		service.accountHistoryPath = filepath.Join(configDir, "sysentra", "accounts.json")
	}
	return service
}

func (s *SystemService) ListAccounts() (AccountsView, error) {
	users, groups, err := s.accounts.List()
	if err != nil {
		return AccountsView{}, err
	}
	statuses, err := s.services.AccountStatus(context.Background())
	if err != nil {
		return AccountsView{}, err
	}
	statusByName := make(map[string]accounts.Status, len(statuses))
	for _, status := range statuses {
		statusByName[status.Name] = status
	}
	view := AccountsView{Users: make([]UserView, len(users)), Groups: make([]GroupView, len(groups))}
	for index, user := range users {
		status := statusByName[user.Name]
		primary := ""
		for _, group := range groups {
			if group.GID == user.GID {
				primary = group.Name
				break
			}
		}
		view.Users[index] = UserView{Name: user.Name, UID: user.UID, GID: user.GID, Home: user.Home, Shell: user.Shell, Groups: user.Groups, Removable: user.UID >= 1000, PrimaryGroup: primary, Password: status.Password, Processes: status.Processes, Sessions: status.Sessions}
	}
	for index, group := range groups {
		view.Groups[index] = GroupView{Name: group.Name, GID: group.GID, Members: group.Members, Removable: group.GID >= 1000}
	}
	return view, nil
}

func (s *SystemService) ChangeAccount(operation, user, group string) (AccountsView, error) {
	request := security.AccountRequest{Version: security.APIVersion, Operation: security.AccountOperation(operation), User: user, Group: group}
	return s.changeAccount(request)
}

func (s *SystemService) AssignAccountGroups(user string, groups []string) (AccountsView, error) {
	request := security.AccountRequest{Version: security.APIVersion, Operation: security.MembershipAdd, User: user, Groups: groups}
	return s.changeAccount(request)
}

func (s *SystemService) CreateUser(user string, createHome bool, home string) (AccountsView, error) {
	request := security.AccountRequest{Version: security.APIVersion, Operation: security.UserAdd, User: user, CreateHome: createHome, Home: home}
	return s.changeAccount(request)
}

func (s *SystemService) EditUser(user, home, shell string, moveHome bool) (AccountsView, error) {
	return s.changeAccount(security.AccountRequest{Version: security.APIVersion, Operation: security.UserEdit, User: user, Home: home, Shell: shell, MoveHome: moveHome})
}

func (s *SystemService) changeAccount(request security.AccountRequest) (AccountsView, error) {
	if err := request.Validate(); err != nil {
		return AccountsView{}, err
	}
	if err := s.services.ChangeAccount(context.Background(), request); err != nil {
		return AccountsView{}, err
	}
	s.recordAccountHistory(request)
	return s.ListAccounts()
}

func (s *SystemService) GetFirewall(backend string) (FirewallView, error) {
	state, err := s.firewall.ReadBackend(context.Background(), backend)
	if err != nil {
		return FirewallView{}, err
	}
	view := FirewallView{Backend: state.Backend, Installed: state.Installed, Active: state.Active, Unknown: state.Unknown, Policy: state.Policy, Zone: state.Zone, Backends: state.Backends, Rules: make([]FirewallRuleView, len(state.Rules))}
	for index, rule := range state.Rules {
		view.Rules[index] = FirewallRuleView{Kind: rule.Kind, Target: rule.Target, Protocol: rule.Protocol, Direction: rule.Direction, Action: rule.Action, Source: rule.Source, Mutable: rule.Kind == "port" || rule.Kind == "service"}
	}
	return view, nil
}

func (s *SystemService) ChangeFirewall(input FirewallChangeInput) (FirewallView, error) {
	request := security.FirewallRequest{Version: security.APIVersion, Operation: security.FirewallOperation(input.Operation), Backend: input.Backend, Zone: input.Zone, Policy: input.Policy, Direction: input.Direction, Rule: firewallSecurityRule(input.Rule), OldRule: firewallSecurityRule(input.OldRule)}
	if err := request.Validate(); err != nil {
		return FirewallView{}, err
	}
	if err := s.services.ChangeFirewall(context.Background(), request); err != nil {
		return FirewallView{}, err
	}
	return s.GetFirewall(input.Backend)
}

func firewallSecurityRule(rule FirewallRuleView) security.FirewallRule {
	source := rule.Source
	if strings.EqualFold(source, "any") || strings.EqualFold(source, "anywhere") {
		source = "any"
	}
	return security.FirewallRule{Kind: rule.Kind, Target: rule.Target, Protocol: strings.ToLower(rule.Protocol), Direction: strings.ToLower(rule.Direction), Source: source, Action: strings.ToLower(rule.Action)}
}

func (s *SystemService) ListDrivers() (DriverCatalogView, error) {
	catalog, err := s.drivers.List(context.Background())
	if err != nil {
		return DriverCatalogView{}, err
	}
	view := DriverCatalogView{Devices: make([]DriverDeviceView, len(catalog.Devices)), Warnings: catalog.Warnings}
	for index, device := range catalog.Devices {
		view.Devices[index] = DriverDeviceView{Category: device.Category, Bus: device.Bus, ID: device.ID, Name: device.Name, Driver: device.Driver, Status: device.Status(), Evidence: device.Evidence}
	}
	return view, nil
}

func (s *SystemService) ListFirmware() (FirmwareCatalogView, error) {
	catalog, err := s.firmware.List(context.Background())
	if err != nil {
		return FirmwareCatalogView{}, err
	}
	view := FirmwareCatalogView{Available: catalog.Available, Devices: make([]FirmwareDeviceView, len(catalog.Devices)), Warning: catalog.Warning}
	for index, device := range catalog.Devices {
		view.Devices[index] = FirmwareDeviceView{Name: device.Name, Vendor: device.Vendor, Version: device.Version, ID: device.ID, Summary: device.Summary, Updatable: device.Updatable, NewVersion: device.NewVersion, RebootRequired: device.RebootRequired, ShutdownRequired: device.ShutdownRequired, ACRequired: device.ACRequired, UpdateRemote: device.UpdateRemote, UpdateSummary: device.UpdateSummary, UpdateMessage: device.UpdateMessage, UpdateUrgency: device.UpdateUrgency, UpdateSize: device.UpdateSize}
	}
	return view, nil
}

func (s *SystemService) ChangeFirmware(operation, deviceID, version string) (FirmwareActionResultView, error) {
	request := security.FirmwareRequest{Version: security.APIVersion, Operation: security.FirmwareOperation(operation), DeviceID: deviceID, VersionName: version}
	if err := request.Validate(); err != nil {
		return FirmwareActionResultView{}, err
	}
	result, err := s.services.ChangeFirmware(context.Background(), request)
	if err != nil {
		return FirmwareActionResultView{Diagnostics: result.Diagnostics}, err
	}
	catalog, err := s.ListFirmware()
	if err != nil {
		return FirmwareActionResultView{Diagnostics: result.Diagnostics}, err
	}
	return FirmwareActionResultView{Catalog: catalog, Diagnostics: result.Diagnostics}, nil
}

func (s *SystemService) ListServices() ([]ServiceView, error) {
	items, err := s.services.ListServices(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list runit services: %w", err)
	}
	return serviceViews(items), nil
}

func (s *SystemService) GetServiceDetails(scope, name string) (ServiceDetailsView, error) {
	var details services.Details
	var err error
	switch scope {
	case "system":
		details, err = s.services.ServiceDetails(context.Background(), name)
	case "user":
		details, err = s.userServices.Details(name)
	case "turnstile":
		details, err = s.turnstileServices.Details(name)
	default:
		err = errors.New("unsupported service scope")
	}
	if err != nil {
		return ServiceDetailsView{}, err
	}
	return ServiceDetailsView{Definition: details.Definition, RunScript: details.RunScript, LogPath: details.LogPath, Logs: details.Logs, Managed: details.Managed}, nil
}

func (s *SystemService) ChangeService(name, operation string, timeoutSeconds int) ([]ServiceView, error) {
	request := security.ServiceRequest{Version: security.APIVersion, Name: name, Operation: security.ServiceOperation(operation), TimeoutSeconds: timeoutSeconds}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := s.services.ChangeService(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListServices()
}

func (s *SystemService) ListUserServices() ([]ServiceView, error) {
	items, err := s.userServices.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list user services: %w", err)
	}
	return serviceViews(items), nil
}

func (s *SystemService) ChangeUserService(name, operation string, timeoutSeconds int) ([]ServiceView, error) {
	request := security.ServiceRequest{Version: security.APIVersion, Name: name, Operation: security.ServiceOperation(operation), TimeoutSeconds: timeoutSeconds}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := s.userManager.Change(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListUserServices()
}

func (s *SystemService) AddService(scope, name, commandLine, username, group string) ([]ServiceView, error) {
	fields := strings.Fields(commandLine)
	if len(fields) == 0 {
		return nil, errors.New("service command is required")
	}
	command, err := exec.LookPath(fields[0])
	if err != nil {
		return nil, fmt.Errorf("service executable %q was not found", fields[0])
	}
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceAdd, Name: name, Command: command, Arguments: fields[1:], User: strings.TrimSpace(username), Group: strings.TrimSpace(group)}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	switch scope {
	case "system":
		if err := s.services.ChangeService(context.Background(), request); err != nil {
			return nil, err
		}
		return s.ListServices()
	case "user":
		if err := s.userManager.Change(context.Background(), request); err != nil {
			return nil, err
		}
		return s.ListUserServices()
	default:
		return nil, errors.New("services can only be added to system or user scope")
	}
}

func (s *SystemService) ListTurnstileServices() ([]ServiceView, error) {
	items, err := s.turnstileServices.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list turnstile services: %w", err)
	}
	return serviceViews(items), nil
}

func (s *SystemService) ChangeTurnstileService(name, operation string, timeoutSeconds int) ([]ServiceView, error) {
	request := security.ServiceRequest{Version: security.APIVersion, Name: name, Operation: security.ServiceOperation(operation), TimeoutSeconds: timeoutSeconds}
	if err := s.turnstileManager.Change(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListTurnstileServices()
}

func (s *SystemService) ChangeTurnstileReady(operation string, names []string) ([]string, error) {
	return s.turnstileManager.Ready(operation, names)
}

func (s *SystemService) UpdateTurnstileEnvironment(values []string) error {
	return s.turnstileManager.Environment(context.Background(), values)
}

func (s *SystemService) InitializeUserServices() error {
	account, err := user.Current()
	if err != nil {
		return err
	}
	request := security.ServiceRequest{Version: security.APIVersion, Name: account.Username, Operation: security.ServiceInitUser}
	return s.services.ChangeService(context.Background(), request)
}

func serviceViews(items []services.Service) []ServiceView {
	views := make([]ServiceView, len(items))
	for index, item := range items {
		next := "enable"
		if item.Enabled {
			next = "disable"
		}
		views[index] = ServiceView{Name: item.Name, Enabled: item.Enabled, Status: item.State, NextAction: next, PID: item.PID, Command: item.Command, Time: item.Time}
	}
	return views
}

func (s *SystemService) GetSambaServer() (SambaServerView, error) {
	server, err := s.shares.Samba()
	if err != nil {
		return SambaServerView{}, err
	}
	view := SambaServerView{Configured: server.Configured, Shares: make([]SambaShareView, len(server.Shares)), Global: SambaGlobalView{ConfigPath: "/etc/samba/smb.conf", Protocol: server.Global.Protocol, DefaultMask: server.Global.DefaultMask, Printing: server.Global.Printing, MapGuest: server.Global.MapGuest}}
	for index, share := range server.Shares {
		view.Shares[index] = SambaShareView{Name: share.Name, Path: share.Path, Comment: share.Comment, ValidUsers: share.ValidUsers, WriteList: share.WriteList, ReadList: share.ReadList, ForceUser: share.ForceUser, ForceGroup: share.ForceGroup, CreateMask: share.CreateMask, DirectoryMask: share.DirectoryMask, ReadOnly: share.ReadOnly, GuestOK: share.GuestOK, Browseable: share.Browseable, FollowSymlinks: share.FollowSymlinks, WideLinks: share.WideLinks}
	}
	if s.services != nil {
		items, err := s.services.ListServices(context.Background())
		if err != nil {
			return SambaServerView{}, fmt.Errorf("read Samba runit services: %w", err)
		}
		for _, item := range serviceViews(items) {
			if item.Name == "smbd" || item.Name == "nmbd" {
				view.Services = append(view.Services, item)
			}
		}
	}
	if s.firewall != nil {
		state, err := s.firewall.ReadBackend(context.Background(), "")
		if err != nil {
			return SambaServerView{}, fmt.Errorf("read Samba firewall state: %w", err)
		}
		view.Global.FirewallBackend = state.Backend
		view.Global.FirewallActive = state.Active
		view.Global.FirewallAllowed = sambaFirewallAllowed(state)
	}
	return view, nil
}

func sambaFirewallAllowed(state firewall.State) bool {
	if state.Backend == "firewalld" {
		for _, rule := range state.Rules {
			if rule.Kind == "service" && rule.Target == "samba" && rule.Action == "allow" {
				return true
			}
		}
		return false
	}
	wanted := map[string]bool{"137/udp": false, "138/udp": false, "139/tcp": false, "445/tcp": false}
	for _, rule := range state.Rules {
		key := rule.Target + "/" + rule.Protocol
		if _, ok := wanted[key]; ok && rule.Action == "allow" {
			wanted[key] = true
		}
	}
	for _, found := range wanted {
		if !found {
			return false
		}
	}
	return true
}

func sambaFirewallConfigured(state firewall.State) bool {
	if state.Backend == "firewalld" {
		return sambaFirewallAllowed(state)
	}
	for _, rule := range state.Rules {
		key := rule.Target + "/" + rule.Protocol
		if (key == "137/udp" || key == "138/udp" || key == "139/tcp" || key == "445/tcp") && rule.Action == "allow" {
			return true
		}
	}
	return false
}

func (s *SystemService) ListNFSClient() ([]NFSMountView, error) {
	items, err := s.shares.NFS()
	if err != nil {
		return nil, err
	}
	views := make([]NFSMountView, len(items))
	for index, item := range items {
		views[index] = NFSMountView{Source: item.Source, Target: item.Target, Type: item.Type, Options: item.Options, Configured: item.Configured, Mounted: item.Mounted}
	}
	return views, nil
}

func (s *SystemService) ListKernels() ([]KernelView, error) {
	items, err := s.kernels.List(context.Background())
	if err != nil {
		return nil, err
	}
	views := make([]KernelView, len(items))
	for index, item := range items {
		views[index] = KernelView{Package: item.Package, Version: item.Version, Series: item.Series, Type: item.Type, Size: item.Size, InstallDate: item.InstallDate, Installed: item.Installed, Running: item.Running, Meta: item.Meta, Headers: item.Headers, CanInstall: item.CanInstall, CanRemove: item.CanRemove}
	}
	return views, nil
}

func (s *SystemService) GetKernelStatus() (KernelStatusView, error) {
	status, err := s.kernels.Status(context.Background())
	if err != nil {
		return KernelStatusView{}, err
	}
	view := KernelStatusView{
		BootMode: status.BootMode, Bootloader: status.Bootloader, InitramfsTool: status.InitramfsTool,
		Boot:            KernelMountView{Target: status.Boot.Target, Source: status.Boot.Source, FileSystem: status.Boot.FileSystem, Size: status.Boot.Size, Available: status.Boot.Available, Mounted: status.Boot.Mounted, Separate: status.Boot.Separate},
		EFI:             KernelMountView{Target: status.EFI.Target, Source: status.EFI.Source, FileSystem: status.EFI.FileSystem, Size: status.EFI.Size, Available: status.EFI.Available, Mounted: status.EFI.Mounted, Separate: status.EFI.Separate},
		SwitchSupported: status.SwitchSupported, SwitchReason: status.SwitchReason, Warnings: status.Warnings,
		Images: make([]KernelBootImageView, len(status.Images)), DKMS: make([]KernelDKMSView, len(status.DKMS)),
	}
	for index, image := range status.Images {
		view.Images[index] = KernelBootImageView{Version: image.Version, Status: image.Status, Size: image.Size, CanPurge: image.CanPurge}
	}
	for index, module := range status.DKMS {
		view.DKMS[index] = KernelDKMSView{Module: module.Module, Version: module.Version, Kernel: module.Kernel, Arch: module.Arch, Status: module.Status, Package: module.Package, Installed: module.Installed}
	}
	return view, nil
}

func (s *SystemService) ReconfigureKernel(packageName string) (PackageTransactionResultView, error) {
	items, err := s.kernels.List(context.Background())
	if err != nil {
		return PackageTransactionResultView{}, err
	}
	installed := false
	for _, item := range items {
		if item.Package == packageName && item.Installed {
			installed = true
			break
		}
	}
	if !installed {
		return PackageTransactionResultView{}, fmt.Errorf("%s is not an installed kernel package", packageName)
	}
	requestID, err := newRequestID()
	if err != nil {
		return PackageTransactionResultView{}, err
	}
	request := security.PackageRequest{Version: security.APIVersion, RequestID: requestID, Operation: security.Reconfigure, Packages: []string{packageName}}
	if _, err := s.services.Plan(context.Background(), request); err != nil {
		return PackageTransactionResultView{}, err
	}
	result, err := s.services.Commit(context.Background(), requestID)
	if err != nil {
		return PackageTransactionResultView{}, err
	}
	return PackageTransactionResultView{RequestID: result.RequestID, Operation: string(result.Operation), Diagnostics: result.Diagnostics}, nil
}

func (s *SystemService) SetDefaultKernel(packageName string) ([]KernelView, error) {
	items, err := s.kernels.List(context.Background())
	if err != nil {
		return nil, err
	}
	version := ""
	for _, item := range items {
		if item.Package == packageName && item.Installed && !item.Running && !item.Meta {
			version = item.Version
			break
		}
	}
	if version == "" {
		return nil, fmt.Errorf("%s is not an installed non-running kernel package", packageName)
	}
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.KernelSetDefault, Value: version}
	if err := s.services.ChangeSystemConfig(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListKernels()
}

func (s *SystemService) ListRepositories() ([]RepositoryView, error) {
	items, err := s.repositories.List(context.Background())
	if err != nil {
		return nil, err
	}
	views := make([]RepositoryView, len(items))
	for index, item := range items {
		views[index] = RepositoryView{URL: item.URL, Priority: item.Priority, Origin: "Effective XBPS configuration", Enabled: item.Enabled, Mutable: item.Mutable}
	}
	return views, nil
}

func (s *SystemService) ListMirrors() (MirrorCatalogView, error) {
	catalog, err := s.mirrors.List(context.Background())
	if err != nil {
		return MirrorCatalogView{}, err
	}
	repositories, repositoryErr := s.repositories.List(context.Background())
	if repositoryErr != nil {
		catalog.Warnings = append(catalog.Warnings, "The currently selected mirror could not be detected.")
	}
	current := currentMirrorBase(repositories)
	view := MirrorCatalogView{Mirrors: make([]MirrorView, len(catalog.Mirrors)), Warnings: catalog.Warnings}
	for index, item := range catalog.Mirrors {
		lastSynced := "Unavailable"
		if item.Age > 0 {
			lastSynced = item.Age.Round(time.Minute).String() + " ago"
		}
		view.Mirrors[index] = MirrorView{URL: item.URL, Region: item.Region, Location: item.Location, Tier: item.Tier, Status: item.Status, LastSynced: lastSynced, Current: strings.TrimSuffix(item.URL, "/") == current}
	}
	return view, nil
}

func (s *SystemService) GetMirrorRecovery() (MirrorRecoveryView, error) {
	value, err := s.recovery.Read()
	return MirrorRecoveryView{Enabled: value.Enabled}, err
}

func (s *SystemService) SetMirrorRecovery(enabled bool) (MirrorRecoveryView, error) {
	value := settings.MirrorRecovery{Enabled: enabled}
	if err := s.recovery.Write(value); err != nil {
		return MirrorRecoveryView{}, err
	}
	return MirrorRecoveryView{Enabled: value.Enabled}, nil
}

func (s *SystemService) ListXBPSConfigRules() ([]XBPSConfigRuleView, error) {
	rules, err := s.xbpsConfig.List()
	if err != nil {
		return nil, err
	}
	views := make([]XBPSConfigRuleView, len(rules))
	for index, rule := range rules {
		views[index] = XBPSConfigRuleView{Kind: rule.Kind, Value: rule.Value, Origin: rule.Origin, Mutable: rule.Mutable}
	}
	return views, nil
}

func (s *SystemService) ChangeXBPSRule(operation, kind, value, oldValue string) ([]XBPSConfigRuleView, error) {
	request := security.XBPSRuleRequest{Version: security.APIVersion, Operation: security.RuleOperation(operation), Kind: security.RuleKind(kind), Value: value, OldValue: oldValue}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := s.services.ChangeXBPSRule(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListXBPSConfigRules()
}

func (s *SystemService) ChangeRepository(operation, value, oldValue string, trustConfirmed bool) ([]RepositoryView, error) {
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SystemConfigOperation("repository-" + operation), Value: value, OldValue: oldValue, TrustConfirmed: trustConfirmed}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := s.services.ChangeSystemConfig(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListRepositories()
}

func (s *SystemService) ChangeMirror(value string) (MirrorCatalogView, error) {
	operation := security.MirrorUse
	if value == "" {
		operation = security.MirrorReset
	}
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: operation, Value: value}
	if err := request.Validate(); err != nil {
		return MirrorCatalogView{}, err
	}
	if err := s.services.ChangeSystemConfig(context.Background(), request); err != nil {
		return MirrorCatalogView{}, err
	}
	return s.ListMirrors()
}

func (s *SystemService) AddSambaShare(input SambaShareInput) (SambaServerView, error) {
	input.Operation = "add"
	return s.ChangeSambaShare(input)
}

func (s *SystemService) ChangeSambaShare(input SambaShareInput) (SambaServerView, error) {
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SystemConfigOperation("samba-" + input.Operation), Value: input.Name, OldValue: input.OldName, Path: input.Path, Comment: input.Comment, ValidUsers: input.ValidUsers, WriteList: input.WriteList, ReadList: input.ReadList, ForceUser: input.ForceUser, ForceGroup: input.ForceGroup, CreateMask: input.CreateMask, DirectoryMask: input.DirectoryMask, ReadOnly: input.ReadOnly, GuestOK: input.GuestOK, Browseable: input.Browseable, FollowSymlinks: input.FollowSymlinks, WideLinks: input.WideLinks}
	if err := request.Validate(); err != nil {
		return SambaServerView{}, err
	}
	if err := s.services.ChangeSystemConfig(context.Background(), request); err != nil {
		return SambaServerView{}, err
	}
	return s.GetSambaServer()
}

func (s *SystemService) ChangeSambaGlobal(input SambaGlobalInput) (SambaServerView, error) {
	before, err := s.shares.Samba()
	if err != nil {
		return SambaServerView{}, err
	}
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaGlobalSet, Protocol: input.Protocol, DefaultMask: input.DefaultMask, Printing: input.Printing, MapGuest: input.MapGuest}
	if err := request.Validate(); err != nil {
		return SambaServerView{}, err
	}
	if err := s.services.ChangeSystemConfig(context.Background(), request); err != nil {
		return SambaServerView{}, err
	}
	if s.firewall != nil {
		state, err := s.firewall.ReadBackend(context.Background(), "")
		if err != nil {
			return SambaServerView{}, err
		}
		firewallChangeNeeded := input.FirewallAllowed && !sambaFirewallAllowed(state) || !input.FirewallAllowed && sambaFirewallConfigured(state)
		if state.Active && firewallChangeNeeded {
			if err := s.changeSambaFirewall(state, input.FirewallAllowed); err != nil {
				rollback := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaGlobalSet, Protocol: before.Global.Protocol, DefaultMask: before.Global.DefaultMask, Printing: before.Global.Printing, MapGuest: before.Global.MapGuest}
				_ = s.services.ChangeSystemConfig(context.Background(), rollback)
				return SambaServerView{}, err
			}
		}
	}
	return s.GetSambaServer()
}

func (s *SystemService) changeSambaFirewall(state firewall.State, allow bool) error {
	operation := security.FirewallRuleAdd
	rollback := security.FirewallRuleRemove
	if !allow {
		operation, rollback = rollback, operation
	}
	rules := []security.FirewallRule{{Kind: "service", Target: "samba", Action: "allow"}}
	if state.Backend == "ufw" {
		rules = []security.FirewallRule{
			{Kind: "port", Target: "137", Protocol: "udp", Direction: "in", Source: "any", Action: "allow"},
			{Kind: "port", Target: "138", Protocol: "udp", Direction: "in", Source: "any", Action: "allow"},
			{Kind: "port", Target: "139", Protocol: "tcp", Direction: "in", Source: "any", Action: "allow"},
			{Kind: "port", Target: "445", Protocol: "tcp", Direction: "in", Source: "any", Action: "allow"},
		}
	}
	filtered := rules[:0]
	for _, rule := range rules {
		present := false
		for _, existing := range state.Rules {
			present = present || existing.Kind == rule.Kind && existing.Target == rule.Target && existing.Protocol == rule.Protocol && existing.Action == "allow"
		}
		if present != allow {
			filtered = append(filtered, rule)
		}
	}
	rules = filtered
	applied := make([]security.FirewallRule, 0, len(rules))
	for _, rule := range rules {
		request := security.FirewallRequest{Version: security.APIVersion, Operation: operation, Backend: state.Backend, Zone: state.Zone, Rule: rule}
		if err := s.services.ChangeFirewall(context.Background(), request); err != nil {
			for index := len(applied) - 1; index >= 0; index-- {
				_ = s.services.ChangeFirewall(context.Background(), security.FirewallRequest{Version: security.APIVersion, Operation: rollback, Backend: state.Backend, Zone: state.Zone, Rule: applied[index]})
			}
			return fmt.Errorf("update Samba firewall access: %w", err)
		}
		applied = append(applied, rule)
	}
	return nil
}

func (s *SystemService) AddNFSMount(source, target, filesystem, options string) ([]NFSMountView, error) {
	return s.ChangeNFSMount("add", source, "", target, "", filesystem, options)
}

func (s *SystemService) ChangeNFSMount(operation, source, oldSource, target, oldTarget, filesystem, options string) ([]NFSMountView, error) {
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SystemConfigOperation("nfs-" + operation), Value: source, OldValue: oldSource, Path: target, OldPath: oldTarget, Type: filesystem, Options: options}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := s.services.ChangeSystemConfig(context.Background(), request); err != nil {
		return nil, err
	}
	return s.ListNFSClient()
}

func currentMirrorBase(items []repositories.Repository) string {
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if index := strings.Index(item.URL, "/current"); index > 0 {
			return strings.TrimSuffix(item.URL[:index], "/")
		}
	}
	return ""
}
