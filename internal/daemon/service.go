package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"sysentra/internal/security"
	"sysentra/internal/transactions"
)

const MaxRequestBytes = 64 << 10
const maxPendingPlans = 128

type Authorizer interface {
	Authorize(context.Context, string, string, bool) error
}

type Service struct {
	authorizer Authorizer
	planner    Planner
	executor   Executor
	audit      AuditSink
	mu         sync.Mutex
	plans      map[string]pendingPlan
}

type Planner interface {
	Plan(context.Context, security.PackageRequest) (transactions.Plan, error)
}

type Executor interface {
	Commit(context.Context, transactions.Plan) (transactions.Result, error)
}

type pendingPlan struct {
	sender string
	plan   transactions.Plan
}

func NewService(authorizer Authorizer, planner Planner, executor Executor, audits ...AuditSink) *Service {
	audit := AuditSink(discardAuditSink{})
	if len(audits) > 0 && audits[0] != nil {
		audit = audits[0]
	}
	return &Service{authorizer: authorizer, planner: planner, executor: executor, audit: audit, plans: make(map[string]pendingPlan)}
}

func (s *Service) PlanPackageRequest(ctx context.Context, sender, payload string) (transactions.Plan, error) {
	if sender == "" {
		s.recordAudit("plan", "rejected", sender, security.PackageRequest{})
		return transactions.Plan{}, errors.New("missing D-Bus sender")
	}
	request, err := decodePackageRequest(payload)
	if err != nil {
		s.recordAudit("plan", "rejected", sender, security.PackageRequest{})
		return transactions.Plan{}, err
	}
	plan, err := s.planner.Plan(ctx, request)
	if err != nil {
		s.recordAudit("plan", "failed", sender, request)
		return transactions.Plan{}, err
	}
	plan.Request = request
	now := time.Now()
	if !now.Before(plan.ExpiresAt) {
		s.recordAudit("plan", "rejected", sender, request)
		return transactions.Plan{}, errors.New("planner returned an expired plan")
	}
	s.mu.Lock()
	for id, pending := range s.plans {
		if !now.Before(pending.plan.ExpiresAt) {
			delete(s.plans, id)
		}
	}
	if _, exists := s.plans[request.RequestID]; exists {
		s.mu.Unlock()
		s.recordAudit("plan", "rejected", sender, request)
		return transactions.Plan{}, errors.New("request ID already has a pending plan")
	}
	if len(s.plans) >= maxPendingPlans {
		s.mu.Unlock()
		s.recordAudit("plan", "rejected", sender, request)
		return transactions.Plan{}, errors.New("too many pending transaction plans")
	}
	s.plans[request.RequestID] = pendingPlan{sender: sender, plan: plan}
	s.mu.Unlock()
	s.recordAudit("plan", "planned", sender, request)
	return plan, nil
}

func (s *Service) CommitPackage(ctx context.Context, sender, requestID string) (transactions.Result, error) {
	s.mu.Lock()
	pending, exists := s.plans[requestID]
	if !exists || pending.sender != sender {
		s.mu.Unlock()
		s.recordAudit("commit", "rejected", sender, security.PackageRequest{RequestID: requestID})
		return transactions.Result{}, errors.New("transaction plan not found")
	}
	delete(s.plans, requestID)
	s.mu.Unlock()

	if err := s.authorizer.Authorize(ctx, sender, security.PackageManagementAction, true); err != nil {
		s.recordAudit("commit", "denied", sender, pending.plan.Request)
		return transactions.Result{}, err
	}
	result, err := s.executor.Commit(ctx, pending.plan)
	if err != nil {
		s.recordAudit("commit", "failed", sender, pending.plan.Request)
		return transactions.Result{}, err
	}
	s.recordAudit("commit", "committed", sender, pending.plan.Request)
	return result, nil
}

func (s *Service) recordAudit(stage, outcome, sender string, request security.PackageRequest) {
	s.audit.Record(AuditEvent{Time: time.Now().UTC(), Stage: stage, Outcome: outcome, Sender: sender, RequestID: request.RequestID, Operation: request.Operation, Packages: append([]string(nil), request.Packages...)})
}

func decodePackageRequest(payload string) (security.PackageRequest, error) {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return security.PackageRequest{}, errors.New("invalid request size")
	}
	var request security.PackageRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return security.PackageRequest{}, fmt.Errorf("decode package request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return security.PackageRequest{}, errors.New("request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return security.PackageRequest{}, err
	}
	return request, nil
}
