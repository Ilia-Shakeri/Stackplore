package domain

import (
	"fmt"
	"time"
)

type AlertTransition struct {
	ID            AlertTransitionID
	AlertID       AlertID
	From          *AlertState
	To            AlertState
	OccurredAt    time.Time
	Actor         AuditActor
	ReasonCode    ReasonCode
	RequestID     *RequestID
	EvidenceRange TimeRange
}

func (transition AlertTransition) Validate() error {
	if err := transition.ID.Validate(); err != nil {
		return err
	}
	if err := transition.AlertID.Validate(); err != nil {
		return err
	}
	if transition.From != nil {
		if err := transition.From.Validate(); err != nil {
			return err
		}
		if *transition.From == transition.To {
			return fmt.Errorf("alert transition states must differ")
		}
	}
	if err := transition.To.Validate(); err != nil {
		return err
	}
	if err := validateUTCTime("alert transition time", transition.OccurredAt); err != nil {
		return err
	}
	if transition.Actor.Kind != AuditActorSystem && transition.Actor.Kind != AuditActorUser {
		return fmt.Errorf("alert transition actor must be a system or user")
	}
	if err := transition.Actor.Validate(); err != nil {
		return err
	}
	if err := transition.ReasonCode.Validate(); err != nil {
		return err
	}
	if transition.RequestID != nil {
		if err := transition.RequestID.Validate(); err != nil {
			return err
		}
	}
	if err := transition.EvidenceRange.Validate(); err != nil {
		return err
	}
	if transition.EvidenceRange.End.After(transition.OccurredAt) {
		return fmt.Errorf("alert transition evidence cannot follow transition time")
	}
	return nil
}

type Notification struct {
	ID          NotificationID
	SourceID    *SourceID
	AlertID     *AlertID
	ResourceID  *ResourceID
	Kind        ReasonCode
	Severity    Severity
	SummaryCode ReasonCode
	CreatedAt   time.Time
	Synthetic   bool
}

func (notification Notification) Validate() error {
	if err := notification.ID.Validate(); err != nil {
		return err
	}
	if notification.SourceID != nil {
		if err := notification.SourceID.Validate(); err != nil {
			return err
		}
	}
	if notification.AlertID != nil {
		if err := notification.AlertID.Validate(); err != nil {
			return err
		}
	}
	if notification.ResourceID != nil {
		if err := notification.ResourceID.Validate(); err != nil {
			return err
		}
	}
	if err := notification.Kind.Validate(); err != nil {
		return err
	}
	if err := notification.Severity.Validate(); err != nil {
		return err
	}
	if err := notification.SummaryCode.Validate(); err != nil {
		return err
	}
	if (notification.AlertID != nil || notification.ResourceID != nil) && notification.SourceID == nil {
		return fmt.Errorf("linked notification requires a source")
	}
	if notification.SourceID == nil && notification.Synthetic {
		return fmt.Errorf("synthetic notification requires a source")
	}
	return validateUTCTime("notification creation time", notification.CreatedAt)
}

type NotificationPage struct {
	Notifications []Notification
	Page          PageInfo
}

func (page NotificationPage) ValidateFor(request PageRequest) error {
	if err := page.Page.ValidateFor(request); err != nil {
		return err
	}
	if len(page.Notifications) > request.Limit {
		return fmt.Errorf("notification page exceeds requested limit")
	}
	seen := make(map[NotificationID]struct{}, len(page.Notifications))
	for index, notification := range page.Notifications {
		if err := notification.Validate(); err != nil {
			return err
		}
		if _, exists := seen[notification.ID]; exists {
			return fmt.Errorf("notification page contains a duplicate notification")
		}
		if index > 0 {
			previous := page.Notifications[index-1]
			if previous.CreatedAt.Before(notification.CreatedAt) ||
				(previous.CreatedAt.Equal(notification.CreatedAt) && string(previous.ID) > string(notification.ID)) {
				return fmt.Errorf("notification page must use descending creation time and ascending ID order")
			}
		}
		seen[notification.ID] = struct{}{}
	}
	return nil
}

type AlertMutation struct {
	Alert                Alert
	DedupeKey            string
	ExpectedUpdatedAt    *time.Time
	ExpectedState        *AlertState
	AcknowledgedByUserID *UserID
	Transition           *AlertTransition
	Notification         *Notification
}

func (mutation AlertMutation) Validate() error {
	if err := mutation.Alert.Validate(); err != nil {
		return err
	}
	if err := validateToken("alert dedupe key", mutation.DedupeKey, maxIDBytes); err != nil {
		return err
	}
	creating := mutation.ExpectedUpdatedAt == nil
	if creating {
		if mutation.ExpectedState != nil {
			return fmt.Errorf("new alert cannot have an expected state")
		}
		if mutation.Alert.State != AlertPending && mutation.Alert.State != AlertFiring {
			return fmt.Errorf("new alert must start pending or firing")
		}
	} else {
		if err := validateUTCTime("expected alert update time", *mutation.ExpectedUpdatedAt); err != nil {
			return err
		}
		if !mutation.ExpectedUpdatedAt.Before(mutation.Alert.UpdatedAt) {
			return fmt.Errorf("new alert update time must follow expected time")
		}
		if mutation.ExpectedState == nil {
			return fmt.Errorf("alert update requires an expected state")
		}
		if err := mutation.ExpectedState.Validate(); err != nil {
			return err
		}
	}
	if mutation.Alert.AcknowledgedAt != nil {
		if mutation.AcknowledgedByUserID == nil || mutation.AcknowledgedByUserID.Validate() != nil {
			return fmt.Errorf("alert acknowledgement requires a valid user")
		}
	} else if mutation.AcknowledgedByUserID != nil {
		return fmt.Errorf("unacknowledged alert cannot retain an acknowledging user")
	}
	if mutation.Transition != nil {
		if err := mutation.Transition.Validate(); err != nil {
			return err
		}
		if mutation.Transition.AlertID != mutation.Alert.ID || mutation.Transition.To != mutation.Alert.State ||
			creating != (mutation.Transition.From == nil) || mutation.Transition.OccurredAt != mutation.Alert.UpdatedAt {
			return fmt.Errorf("alert transition does not match alert mutation")
		}
	} else if creating {
		return fmt.Errorf("new alert requires an opening transition")
	}
	if !creating {
		stateChanged := *mutation.ExpectedState != mutation.Alert.State
		if stateChanged && (mutation.Transition == nil || mutation.Transition.From == nil ||
			*mutation.Transition.From != *mutation.ExpectedState) {
			return fmt.Errorf("alert state change requires a matching transition")
		}
		if !stateChanged && mutation.Transition != nil {
			return fmt.Errorf("unchanged alert state cannot add a transition")
		}
	}
	if mutation.Notification != nil {
		if mutation.Transition == nil || mutation.Transition.To != AlertFiring && mutation.Transition.To != AlertResolved {
			return fmt.Errorf("notification requires a firing or resolved transition")
		}
		if err := mutation.Notification.Validate(); err != nil {
			return err
		}
		if mutation.Notification.SourceID == nil || *mutation.Notification.SourceID != mutation.Alert.SourceID ||
			mutation.Notification.AlertID == nil || *mutation.Notification.AlertID != mutation.Alert.ID ||
			!sameOptionalResource(mutation.Notification.ResourceID, mutation.Alert.ResourceID) ||
			mutation.Notification.CreatedAt != mutation.Transition.OccurredAt ||
			mutation.Notification.Synthetic != mutation.Alert.Synthetic {
			return fmt.Errorf("notification does not match alert mutation")
		}
	}
	return nil
}

func sameOptionalResource(left, right *ResourceID) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
