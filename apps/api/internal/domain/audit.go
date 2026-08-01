package domain

import (
	"fmt"
	"time"
)

type AuditActorKind string

const (
	AuditActorUser     AuditActorKind = "user"
	AuditActorOperator AuditActorKind = "operator"
	AuditActorSystem   AuditActorKind = "system"
)

type AuditActor struct {
	Kind AuditActorKind
	ID   string
}

func (actor AuditActor) Validate() error {
	switch actor.Kind {
	case AuditActorUser:
		return validateToken("audit actor ID", actor.ID, maxIDBytes)
	case AuditActorOperator:
		if actor.ID == "" {
			return nil
		}
		return validateToken("audit actor ID", actor.ID, maxIDBytes)
	case AuditActorSystem:
		if actor.ID != "" {
			return fmt.Errorf("system audit actor must not have an ID")
		}
		return nil
	default:
		return fmt.Errorf("unknown audit actor kind %q", actor.Kind)
	}
}

type AuditAction string

func (action AuditAction) Validate() error {
	return validateNamespacedCode("audit action", string(action))
}

type AuditTargetKind string

func (kind AuditTargetKind) Validate() error {
	return validateNamespacedCode("audit target kind", string(kind))
}

type AuditTarget struct {
	Kind AuditTargetKind
	ID   string
}

func (target AuditTarget) Validate() error {
	if err := target.Kind.Validate(); err != nil {
		return err
	}
	if target.ID == "" {
		return nil
	}
	return validateToken("audit target ID", target.ID, maxIDBytes)
}

type AuditOutcome string

const (
	AuditIntent    AuditOutcome = "intent"
	AuditSuccess   AuditOutcome = "success"
	AuditFailure   AuditOutcome = "failure"
	AuditDenied    AuditOutcome = "denied"
	AuditUncertain AuditOutcome = "uncertain"
)

func (outcome AuditOutcome) Validate() error {
	switch outcome {
	case AuditIntent, AuditSuccess, AuditFailure, AuditDenied, AuditUncertain:
		return nil
	default:
		return fmt.Errorf("unknown audit outcome %q", outcome)
	}
}

type AuditEntry struct {
	ID         AuditEntryID
	OccurredAt time.Time
	Actor      AuditActor
	Action     AuditAction
	Target     AuditTarget
	Outcome    AuditOutcome
	RequestID  RequestID
	ReasonCode ReasonCode
}

func (entry AuditEntry) Validate() error {
	if err := entry.ID.Validate(); err != nil {
		return err
	}
	if err := validateUTCTime("audit occurrence time", entry.OccurredAt); err != nil {
		return err
	}
	if err := entry.Actor.Validate(); err != nil {
		return err
	}
	if err := entry.Action.Validate(); err != nil {
		return err
	}
	if err := entry.Target.Validate(); err != nil {
		return err
	}
	if err := entry.Outcome.Validate(); err != nil {
		return err
	}
	if err := entry.RequestID.Validate(); err != nil {
		return err
	}
	return entry.ReasonCode.Validate()
}
