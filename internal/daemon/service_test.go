package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"sysentra/internal/security"
	"sysentra/internal/transactions"
)

type authorizationStub struct {
	err    error
	sender string
	action string
}

type plannerStub struct{ plan transactions.Plan }

func (p plannerStub) Plan(_ context.Context, request security.PackageRequest) (transactions.Plan, error) {
	p.plan.Request = request
	return p.plan, nil
}

type executorStub struct {
	commits int
	plan    transactions.Plan
}

type streamingExecutorStub struct{ executorStub }

func (e *streamingExecutorStub) CommitStreaming(_ context.Context, plan transactions.Plan, output func(string)) (transactions.Result, error) {
	e.commits++
	e.plan = plan
	output("installing firefox")
	return transactions.Result{RequestID: plan.Request.RequestID, Operation: plan.Request.Operation}, nil
}

type auditStub struct{ events []AuditEvent }

func (a *auditStub) Record(event AuditEvent) { a.events = append(a.events, event) }

func (e *executorStub) Commit(_ context.Context, plan transactions.Plan) (transactions.Result, error) {
	e.commits++
	e.plan = plan
	return transactions.Result{RequestID: plan.Request.RequestID, Operation: plan.Request.Operation}, nil
}

func (a *authorizationStub) Authorize(_ context.Context, sender, action string, interactive bool) error {
	if !interactive {
		return errors.New("interaction was not requested")
	}
	a.sender, a.action = sender, action
	return a.err
}

func TestAuthorizeSessionUsesInteractivePackagePermission(t *testing.T) {
	authorizer := &authorizationStub{}
	service := NewService(authorizer, plannerStub{}, &executorStub{})
	if err := service.Authorize(context.Background(), ":1.42"); err != nil {
		t.Fatal(err)
	}
	if authorizer.sender != ":1.42" || authorizer.action != security.PackageManagementAction {
		t.Fatalf("sender=%q action=%q", authorizer.sender, authorizer.action)
	}
}

func TestAuthorizePackageRequestValidatesBeforeAuthorization(t *testing.T) {
	authorizer := &authorizationStub{}
	service := NewService(authorizer, plannerStub{}, &executorStub{})
	_, err := service.PlanPackageRequest(context.Background(), ":1.42", `{"version":1,"requestId":"request-1234","operation":"shell","packages":["firefox"]}`)
	if err == nil || authorizer.sender != "" {
		t.Fatalf("invalid request reached authorizer: sender=%q err=%v", authorizer.sender, err)
	}
}

func TestCommitPackageIsSenderBoundAuthorizedAndOneShot(t *testing.T) {
	authorizer := &authorizationStub{}
	executor := &executorStub{}
	audit := &auditStub{}
	service := NewService(authorizer, plannerStub{plan: transactions.Plan{StateFingerprint: "state-a", ExpiresAt: time.Now().Add(time.Minute)}}, executor, audit)
	if _, err := service.PlanPackageRequest(context.Background(), ":1.42", `{"version":1,"requestId":"request-1234","operation":"install","packages":["firefox"]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CommitPackage(context.Background(), ":1.99", "request-1234"); err == nil || executor.commits != 0 {
		t.Fatalf("different sender committed plan: commits=%d err=%v", executor.commits, err)
	}
	result, err := service.CommitPackage(context.Background(), ":1.42", "request-1234")
	if err != nil || result.RequestID != "request-1234" || executor.commits != 1 || authorizer.sender != ":1.42" {
		t.Fatalf("commit failed: result=%#v commits=%d sender=%q err=%v", result, executor.commits, authorizer.sender, err)
	}
	if _, err := service.CommitPackage(context.Background(), ":1.42", "request-1234"); err == nil || executor.commits != 1 {
		t.Fatalf("plan was reusable: commits=%d err=%v", executor.commits, err)
	}
	if authorizer.action != security.PackageManagementAction {
		t.Fatalf("action = %q", authorizer.action)
	}
	if len(audit.events) != 4 || audit.events[0].Outcome != "planned" || audit.events[2].Outcome != "committed" || audit.events[2].RequestID != "request-1234" || len(audit.events[2].Packages) != 1 {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestCommitPackageFailsClosedOnAuthorizationDenial(t *testing.T) {
	authorizer := &authorizationStub{err: errors.New("denied")}
	executor := &executorStub{}
	audit := &auditStub{}
	service := NewService(authorizer, plannerStub{plan: transactions.Plan{StateFingerprint: "state-a", ExpiresAt: time.Now().Add(time.Minute)}}, executor, audit)
	if _, err := service.PlanPackageRequest(context.Background(), ":1.42", `{"version":1,"requestId":"request-1234","operation":"install","packages":["firefox"]}`); err != nil {
		t.Fatal(err)
	}
	_, err := service.CommitPackage(context.Background(), ":1.42", "request-1234")
	if err == nil || executor.commits != 0 {
		t.Fatalf("denied request reached executor: commits=%d err=%v", executor.commits, err)
	}
	if len(audit.events) != 2 || audit.events[1].Outcome != "denied" {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestCommitPackageStreamsExecutorOutput(t *testing.T) {
	executor := &streamingExecutorStub{}
	service := NewService(&authorizationStub{}, plannerStub{plan: transactions.Plan{ExpiresAt: time.Now().Add(time.Minute)}}, executor)
	if _, err := service.PlanPackageRequest(context.Background(), ":1.42", `{"version":1,"requestId":"request-1234","operation":"install","packages":["firefox"]}`); err != nil {
		t.Fatal(err)
	}
	var lines []string
	if _, err := service.CommitPackageStreaming(context.Background(), ":1.42", "request-1234", func(line string) { lines = append(lines, line) }); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "installing firefox" {
		t.Fatalf("lines = %#v", lines)
	}
}
