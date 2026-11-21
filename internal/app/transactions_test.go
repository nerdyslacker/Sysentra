package app

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sysentra/internal/flatpakplan"
	"sysentra/internal/runner"
	"sysentra/internal/security"
	"sysentra/internal/transactions"
)

type transactionClientStub struct {
	request security.PackageRequest
	plan    transactions.Plan
}

func (s *transactionClientStub) AuthorizeSession(context.Context) error { return nil }

type transactionRunnerStub struct {
	calls  [][]string
	result runner.Result
}

type flatpakPlannerStub struct{ changes []flatpakplan.Change }

func (stub flatpakPlannerStub) Preview(context.Context, []string, string, string, string) ([]flatpakplan.Change, error) {
	return stub.changes, nil
}

func (s *transactionRunnerStub) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	s.calls = append(s.calls, append([]string{name}, args...))
	return s.result, nil
}

func (s *transactionClientStub) Plan(_ context.Context, request security.PackageRequest) (transactions.Plan, error) {
	s.request = request
	s.plan.Request = request
	return s.plan, nil
}

func (s *transactionClientStub) Commit(_ context.Context, requestID string) (transactions.Result, error) {
	return transactions.Result{RequestID: requestID, Operation: s.request.Operation}, nil
}

func TestPackageTransactionServicePlansAndCommitsXBPSOperation(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0), Diagnostics: []string{"firefox install"}}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	plan, err := service.PreviewPackage("xbps", "firefox", "install", "")
	if err != nil {
		t.Fatal(err)
	}
	if client.request.Operation != security.Install || len(client.request.Packages) != 1 || client.request.Packages[0] != "firefox" || plan.RequestID == "" || len(plan.Diagnostics) != 1 {
		t.Fatalf("request=%#v plan=%#v", client.request, plan)
	}
	result, err := service.CommitPackage(plan.RequestID)
	if err != nil || result.RequestID != plan.RequestID || result.Operation != "install" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestPackageTransactionServicePersistsBoundedHistory(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0)}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	service.historyPath = filepath.Join(t.TempDir(), "transactions.json")
	plan, err := service.PreviewPackage("xbps", "firefox", "install", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CommitPackage(plan.RequestID); err != nil {
		t.Fatal(err)
	}
	history, err := service.ListTransactionHistory()
	if err != nil || len(history) != 1 || history[0].Operation != "install" || history[0].Status != "completed" {
		t.Fatalf("history=%#v err=%v", history, err)
	}
}

func TestPackageTransactionServicePlansDatabaseSync(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0)}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	plan, err := service.PreviewDatabaseSync()
	if err != nil {
		t.Fatal(err)
	}
	if client.request.Operation != security.SyncDatabase || len(client.request.Packages) != 0 || plan.Operation != string(security.SyncDatabase) {
		t.Fatalf("request=%#v plan=%#v", client.request, plan)
	}
}

func TestPackageTransactionServicePlansAndCommitsFlatpakOperation(t *testing.T) {
	commandRunner := &transactionRunnerStub{result: runner.Result{Stdout: []byte("Installing org.example.App\nComplete.\n")}}
	service := NewPackageTransactionService(&transactionClientStub{}, commandRunner)
	plan, err := service.PreviewPackage("flatpak", "org.example.App", "install", "user")
	if err != nil || plan.Operation != "install" || len(plan.Changes) != 1 {
		t.Fatalf("plan=%#v err=%v", plan, err)
	}
	result, err := service.CommitPackage(plan.RequestID)
	if err != nil || result.Operation != "install" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !reflect.DeepEqual(result.Diagnostics, []string{"Installing org.example.App", "Complete."}) {
		t.Fatalf("diagnostics=%#v", result.Diagnostics)
	}
	want := []string{"flatpak", "install", "--user", "--assumeyes", "--noninteractive", "flathub", "org.example.App"}
	if len(commandRunner.calls) != 1 || !reflect.DeepEqual(commandRunner.calls[0], want) {
		t.Fatalf("calls=%v want %v", commandRunner.calls, want)
	}
}

func TestPackageTransactionServiceCleansUnusedFlatpaks(t *testing.T) {
	commandRunner := &transactionRunnerStub{}
	service := NewPackageTransactionService(&transactionClientStub{}, commandRunner)
	plan, err := service.PreviewFlatpakCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CommitPackage(plan.RequestID); err != nil {
		t.Fatal(err)
	}
	want := []string{"flatpak", "uninstall", "--user", "--unused", "--assumeyes", "--noninteractive"}
	if len(commandRunner.calls) != 1 || !reflect.DeepEqual(commandRunner.calls[0], want) {
		t.Fatalf("calls=%v want %v", commandRunner.calls, want)
	}
}

func TestPackageTransactionServicePreservesFlatpakSystemScope(t *testing.T) {
	commandRunner := &transactionRunnerStub{}
	service := NewPackageTransactionService(&transactionClientStub{}, commandRunner)
	plan, err := service.PreviewPackage("flatpak", "org.example.App", "update", "system")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CommitPackage(plan.RequestID); err != nil {
		t.Fatal(err)
	}
	want := []string{"flatpak", "update", "--system", "--assumeyes", "--noninteractive", "org.example.App"}
	if len(commandRunner.calls) != 1 || !reflect.DeepEqual(commandRunner.calls[0], want) {
		t.Fatalf("calls=%v want %v", commandRunner.calls, want)
	}
}

func TestPackageTransactionServiceShowsResolvedFlatpakDependenciesAndSizes(t *testing.T) {
	planner := flatpakPlannerStub{changes: []flatpakplan.Change{
		{Ref: "runtime/org.gnome.Platform/x86_64/51", Action: "install", NewVersion: "51.2", DownloadSize: 400, InstalledSize: 900},
		{Ref: "app/org.gnome.Calculator/x86_64/stable", Action: "install", NewVersion: "51.1", DownloadSize: 20, InstalledSize: 40},
	}}
	service := NewPackageTransactionService(&transactionClientStub{}, &transactionRunnerStub{}, planner)
	plan, err := service.PreviewPackage("flatpak", "org.gnome.Calculator", "install", "system")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 2 || plan.Changes[0].Name != "org.gnome.Platform" || plan.Changes[1].NewVersion != "51.1" || plan.DownloadSize != 420 || plan.InstalledSize != 940 {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestPackageTransactionServiceShowsFlatpakRemovalVersionAndFreedSize(t *testing.T) {
	planner := flatpakPlannerStub{changes: []flatpakplan.Change{{Ref: "app/org.example.App/x86_64/stable", Action: "uninstall", CurrentVersion: "2.4", InstalledSize: 1024}}}
	service := NewPackageTransactionService(&transactionClientStub{}, &transactionRunnerStub{}, planner)
	plan, err := service.PreviewPackage("flatpak", "org.example.App", "remove", "system")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Version != "2.4" || plan.Changes[0].NewVersion != "" || plan.InstalledSize != 1024 {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestPackageTransactionServicePlansSelectedPackages(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0)}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	plan, err := service.PreviewPackages("xbps", []string{"firefox", "curl"}, "install", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.RequestID == "" || len(client.request.Packages) != 2 || client.request.Packages[0] != "firefox" || client.request.Packages[1] != "curl" {
		t.Fatalf("request=%#v plan=%#v", client.request, plan)
	}
}

func TestPackageTransactionServicePlansSystemUpgradeWithoutPackageNames(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0)}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	plan, err := service.PreviewSystemUpgrade()
	if err != nil {
		t.Fatal(err)
	}
	if plan.RequestID == "" || client.request.Operation != security.SystemUpgrade || len(client.request.Packages) != 0 {
		t.Fatalf("request=%#v plan=%#v", client.request, plan)
	}
}

func TestPackageTransactionServicePlansMaintenanceOperation(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0)}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	plan, err := service.PreviewMaintenance("force-remove", []string{"firefox"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RequestID == "" || client.request.Operation != security.ForceRemove || len(client.request.Packages) != 1 {
		t.Fatalf("request=%#v plan=%#v", client.request, plan)
	}
}

func TestPackageTransactionServicePlansCachedDowngrade(t *testing.T) {
	client := &transactionClientStub{plan: transactions.Plan{ExpiresAt: time.Unix(100, 0)}}
	service := NewPackageTransactionService(client, &transactionRunnerStub{})
	if _, err := service.PreviewDowngrade("firefox", "129.0_1"); err != nil {
		t.Fatal(err)
	}
	if client.request.Operation != security.Downgrade || client.request.TargetVersion != "129.0_1" {
		t.Fatalf("request=%#v", client.request)
	}
}
