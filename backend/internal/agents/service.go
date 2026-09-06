// Package agents contains the trip-scoped AI agent runtime.
//
// Future assistant scope:
//   - One agent per trip, configured with a list of capabilities.
//   - User posts a message -> Service.PostMessage builds trip-scoped context,
//     invokes the LLM with the allowed tool set, and persists any tool calls
//     as rows in agent_action_proposals (status="pending"). No side effects.
//   - User reviews proposals; ConfirmProposal dispatches the call through the
//     ToolRegistry which routes to the owning module's Service. This is the
//     only place where the agent can mutate trip state, and every dispatch
//     writes an audit_log row.
//
// Guardrails baked in by design:
//  1. Context is built from the requesting user's view only.
//  2. Every tool is registered with required role + destructive flag.
//  3. Proposals are the only persistence path for tool calls; execution is
//     a separate, explicitly-authorized step.
//  4. Deletes/edits are not exposed until the product has review controls.
//
// This file is the type and interface shell. Routes are intentionally not
// wired until the assistant milestone is implemented.
package agents

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "pending"
	ProposalApproved ProposalStatus = "approved"
	ProposalRejected ProposalStatus = "rejected"
	ProposalExecuted ProposalStatus = "executed"
	ProposalFailed   ProposalStatus = "failed"
)

type ActionProposal struct {
	ID      uuid.UUID
	AgentID uuid.UUID
	TripID  uuid.UUID
	Tool    string
	Payload []byte // JSON
	Status  ProposalStatus
}

// Tool describes a capability the agent may propose. The registry must reject
// any tool name the agent emits that isn't registered.
type Tool struct {
	Name         string
	Description  string
	RequiredRole string // "owner" | "admin" | "member"
	Destructive  bool
}

type ToolRegistry interface {
	Register(t Tool, exec ToolExecutor)
	Get(name string) (Tool, ToolExecutor, bool)
}

// ToolExecutor is the only path through which a confirmed proposal can mutate
// state. Implementations call the owning module's Service.
type ToolExecutor func(ctx context.Context, tripID, userID uuid.UUID, payload []byte) (any, error)

// Service is the future public surface used by handlers.
type Service struct{}

func NewService() *Service { return &Service{} }

// ErrNotImplemented signals that the skeleton intentionally does not run the
// agent yet. Routes must wait for the assistant milestone.
var ErrNotImplemented = errors.New("agents: not implemented")
