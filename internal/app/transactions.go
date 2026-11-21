package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"sysentra/internal/flatpakplan"
	"sysentra/internal/runner"
	"sysentra/internal/security"
	"sysentra/internal/transactions"
)

var transactionRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$`)
var flatpakPackageID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,255}$`)

type packageTransactionClient interface {
	AuthorizeSession(context.Context) error
	Plan(context.Context, security.PackageRequest) (transactions.Plan, error)
	Commit(context.Context, string) (transactions.Result, error)
}

func (s *PackageTransactionService) AuthorizeSession() error {
	return s.client.AuthorizeSession(context.Background())
}

type flatpakTransactionPlanner interface {
	Preview(context.Context, []string, string, string, string) ([]flatpakplan.Change, error)
}

type PackageTransactionPlanView struct {
	RequestID      string                         `json:"requestId"`
	Operation      string                         `json:"operation"`
	ExpiresAt      string                         `json:"expiresAt"`
	Diagnostics    []string                       `json:"diagnostics"`
	Changes        []PackageTransactionChangeView `json:"changes"`
	DownloadSize   int64                          `json:"downloadSize"`
	InstalledSize  int64                          `json:"installedSize"`
	AvailableSpace int64                          `json:"availableSpace"`
}

type PackageTransactionChangeView struct {
	Name         string `json:"name"`
	Action       string `json:"action"`
	Version      string `json:"version"`
	NewVersion   string `json:"newVersion"`
	DownloadSize int64  `json:"downloadSize"`
}

type PackageTransactionResultView struct {
	RequestID   string   `json:"requestId"`
	Operation   string   `json:"operation"`
	Diagnostics []string `json:"diagnostics"`
}

type flatpakTransaction struct {
	operation string
	command   []string
	expires   time.Time
}

type PackageTransactionService struct {
	client      packageTransactionClient
	runner      runner.Runner
	mu          sync.Mutex
	flatpak     map[string]flatpakTransaction
	planned     map[string]string
	planner     flatpakTransactionPlanner
	historyMu   sync.Mutex
	historyPath string
}

func NewPackageTransactionService(client packageTransactionClient, commandRunner runner.Runner, planners ...flatpakTransactionPlanner) *PackageTransactionService {
	service := &PackageTransactionService{client: client, runner: commandRunner, flatpak: make(map[string]flatpakTransaction), planned: make(map[string]string)}
	if len(planners) > 0 {
		service.planner = planners[0]
	}
	return service
}

func (s *PackageTransactionService) PreviewPackage(backendID, packageID, operation, scope string) (PackageTransactionPlanView, error) {
	return s.PreviewPackages(backendID, []string{packageID}, operation, scope)
}

func (s *PackageTransactionService) PreviewPackages(backendID string, packageIDs []string, operation, scope string) (PackageTransactionPlanView, error) {
	if backendID == "flatpak" {
		return s.previewFlatpak(packageIDs, operation, scope)
	}
	if backendID != "xbps" {
		return PackageTransactionPlanView{}, errors.New("unsupported package backend")
	}
	requestID, err := newRequestID()
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	request := security.PackageRequest{Version: security.APIVersion, RequestID: requestID, Operation: security.Operation(operation), Packages: packageIDs}
	if err := request.Validate(); err != nil {
		return PackageTransactionPlanView{}, err
	}
	plan, err := s.client.Plan(context.Background(), request)
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	return s.rememberPlan(packagePlanView(plan)), nil
}

func (s *PackageTransactionService) PreviewFlatpakCleanup() (PackageTransactionPlanView, error) {
	return s.storeFlatpakPlan("cleanup-unused", []string{"flatpak", "uninstall", "--user", "--unused", "--assumeyes", "--noninteractive"}, nil)
}

func (s *PackageTransactionService) previewFlatpak(packageIDs []string, operation, scope string) (PackageTransactionPlanView, error) {
	if len(packageIDs) == 0 {
		return PackageTransactionPlanView{}, errors.New("at least one Flatpak package is required")
	}
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "system" {
		return PackageTransactionPlanView{}, errors.New("invalid Flatpak installation scope")
	}
	scopeFlag := "--" + scope
	args := []string{}
	switch operation {
	case "install":
		args = []string{"flatpak", "install", scopeFlag, "--assumeyes", "--noninteractive", "flathub"}
	case "remove":
		args = []string{"flatpak", "uninstall", scopeFlag, "--assumeyes", "--noninteractive"}
	case "update":
		args = []string{"flatpak", "update", scopeFlag, "--assumeyes", "--noninteractive"}
	default:
		return PackageTransactionPlanView{}, errors.New("unsupported Flatpak operation")
	}
	changes := make([]PackageTransactionChangeView, len(packageIDs))
	for index, packageID := range packageIDs {
		if !flatpakPackageID.MatchString(packageID) {
			return PackageTransactionPlanView{}, errors.New("invalid Flatpak package ID")
		}
		args = append(args, packageID)
		changes[index] = PackageTransactionChangeView{Name: packageID, Action: operation}
	}
	if s.planner == nil {
		return s.storeFlatpakPlan(operation, args, changes)
	}
	resolved, err := s.planner.Preview(context.Background(), packageIDs, operation, scope, "flathub")
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	changes = make([]PackageTransactionChangeView, len(resolved))
	var downloadSize, installedSize int64
	for index, change := range resolved {
		action := change.Action
		if action == "uninstall" {
			action = "remove"
		}
		parts := strings.Split(change.Ref, "/")
		name, newVersion := change.Ref, change.NewVersion
		if len(parts) >= 4 {
			name = parts[1]
			if newVersion == "" && operation == "install" {
				newVersion = parts[len(parts)-1]
			}
		}
		changes[index] = PackageTransactionChangeView{Name: name, Action: action, Version: change.CurrentVersion, NewVersion: newVersion, DownloadSize: change.DownloadSize}
		downloadSize += change.DownloadSize
		installedSize += change.InstalledSize
	}
	plan, err := s.storeFlatpakPlan(operation, args, changes)
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	plan.DownloadSize, plan.InstalledSize, plan.AvailableSpace = downloadSize, installedSize, availableDiskSpace("/")
	plan.Diagnostics = []string{"Flatpak installed size is a maximum; shared or cached objects can reduce the actual disk-space increase."}
	return plan, nil
}

func availableDiskSpace(path string) int64 {
	var state unix.Statfs_t
	if unix.Statfs(path, &state) != nil {
		return 0
	}
	return int64(state.Bavail) * int64(state.Bsize)
}

func (s *PackageTransactionService) storeFlatpakPlan(operation string, command []string, changes []PackageTransactionChangeView) (PackageTransactionPlanView, error) {
	requestID, err := newRequestID()
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	expires := time.Now().Add(5 * time.Minute)
	s.mu.Lock()
	s.flatpak[requestID] = flatpakTransaction{operation: operation, command: command, expires: expires}
	s.planned[requestID] = operation
	s.mu.Unlock()
	return PackageTransactionPlanView{RequestID: requestID, Operation: operation, ExpiresAt: expires.Format(timeLayout), Changes: changes, Diagnostics: []string{"Flatpak installed size is a maximum; shared or cached objects can reduce the actual disk-space increase."}}, nil
}

func (s *PackageTransactionService) PreviewSystemUpgrade() (PackageTransactionPlanView, error) {
	return s.previewPackageOperation(security.SystemUpgrade)
}

func (s *PackageTransactionService) PreviewDatabaseSync() (PackageTransactionPlanView, error) {
	return s.previewPackageOperation(security.SyncDatabase)
}

func (s *PackageTransactionService) PreviewMaintenance(operation string, packageIDs []string) (PackageTransactionPlanView, error) {
	return s.PreviewPackages("xbps", packageIDs, operation, "")
}

func (s *PackageTransactionService) PreviewDowngrade(packageID, targetVersion string) (PackageTransactionPlanView, error) {
	requestID, err := newRequestID()
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	request := security.PackageRequest{Version: security.APIVersion, RequestID: requestID, Operation: security.Downgrade, Packages: []string{packageID}, TargetVersion: targetVersion}
	if err := request.Validate(); err != nil {
		return PackageTransactionPlanView{}, err
	}
	plan, err := s.client.Plan(context.Background(), request)
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	return s.rememberPlan(packagePlanView(plan)), nil
}

func (s *PackageTransactionService) previewPackageOperation(operation security.Operation) (PackageTransactionPlanView, error) {
	requestID, err := newRequestID()
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	request := security.PackageRequest{Version: security.APIVersion, RequestID: requestID, Operation: operation}
	plan, err := s.client.Plan(context.Background(), request)
	if err != nil {
		return PackageTransactionPlanView{}, err
	}
	return s.rememberPlan(packagePlanView(plan)), nil
}

func (s *PackageTransactionService) rememberPlan(plan PackageTransactionPlanView) PackageTransactionPlanView {
	s.mu.Lock()
	s.planned[plan.RequestID] = plan.Operation
	s.mu.Unlock()
	return plan
}

func packagePlanView(plan transactions.Plan) PackageTransactionPlanView {
	changes := make([]PackageTransactionChangeView, len(plan.Changes))
	for index, change := range plan.Changes {
		changes[index] = PackageTransactionChangeView{Name: change.Package, Action: change.Action, Version: change.FromVersion, NewVersion: change.ToVersion, DownloadSize: change.DownloadSize}
	}
	return PackageTransactionPlanView{RequestID: plan.Request.RequestID, Operation: string(plan.Request.Operation), ExpiresAt: plan.ExpiresAt.Format(timeLayout), Diagnostics: plan.Diagnostics, Changes: changes, DownloadSize: plan.DownloadSize, InstalledSize: plan.InstalledSize, AvailableSpace: plan.AvailableSpace}
}

func (s *PackageTransactionService) CommitPackage(requestID string) (PackageTransactionResultView, error) {
	if !transactionRequestID.MatchString(requestID) {
		return PackageTransactionResultView{}, errors.New("invalid transaction request ID")
	}
	s.mu.Lock()
	flatpakPlan, isFlatpak := s.flatpak[requestID]
	operation := s.planned[requestID]
	delete(s.planned, requestID)
	if isFlatpak {
		delete(s.flatpak, requestID)
	}
	s.mu.Unlock()
	if isFlatpak {
		if time.Now().After(flatpakPlan.expires) {
			err := errors.New("Flatpak transaction preview expired")
			s.recordTransaction(requestID, operation, "failed", err.Error())
			return PackageTransactionResultView{}, err
		}
		result, err := s.runner.Run(context.Background(), flatpakPlan.command[0], flatpakPlan.command[1:]...)
		if err != nil {
			message := strings.TrimSpace(string(result.Stderr))
			if message == "" {
				message = err.Error()
			}
			err := fmt.Errorf("Flatpak %s failed: %s", flatpakPlan.operation, message)
			s.recordTransaction(requestID, operation, "failed", err.Error())
			return PackageTransactionResultView{}, err
		}
		s.recordTransaction(requestID, operation, "completed", "")
		return PackageTransactionResultView{RequestID: requestID, Operation: flatpakPlan.operation, Diagnostics: commandOutputLines(result)}, nil
	}
	result, err := s.client.Commit(context.Background(), requestID)
	if err != nil {
		s.recordTransaction(requestID, operation, "failed", err.Error())
		return PackageTransactionResultView{}, err
	}
	s.recordTransaction(requestID, operation, "completed", "")
	return PackageTransactionResultView{RequestID: result.RequestID, Operation: string(result.Operation), Diagnostics: result.Diagnostics}, nil
}

func commandOutputLines(result runner.Result) []string {
	output := strings.TrimSpace(string(result.Stdout))
	if stderr := strings.TrimSpace(string(result.Stderr)); stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}
	if output == "" {
		return []string{}
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 500 {
		lines = lines[:500]
	}
	return lines
}

func newRequestID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "request-" + hex.EncodeToString(buffer), nil
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
