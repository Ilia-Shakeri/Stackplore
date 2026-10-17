package domain

import "time"

const (
	MaximumHistoryCleanupRows = 1000
	HistoryImpactCountCap     = 10001
	MinimumAuditRetentionDays = 90
	AuditCompletionHeadroom   = 256
)

type ApplicationValues struct {
	MetricRetentionHours       int `json:"metric_retention_hours"`
	HealthRetentionHours       int `json:"health_retention_hours"`
	EventRetentionHours        int `json:"event_retention_hours"`
	DeliveryRetentionHours     int `json:"delivery_retention_hours"`
	LogLineLimit               int `json:"log_line_limit"`
	DefaultAlertWindowSeconds  int `json:"default_alert_window_seconds"`
	DefaultAlertPendingSeconds int `json:"default_alert_pending_seconds"`
}

func DefaultApplicationValues() ApplicationValues {
	return ApplicationValues{MetricRetentionHours: 168, HealthRetentionHours: 720, EventRetentionHours: 720,
		DeliveryRetentionHours: 720, LogLineLimit: 200, DefaultAlertWindowSeconds: 300}
}

func (values ApplicationValues) Validate() error {
	for _, hours := range []int{values.MetricRetentionHours, values.HealthRetentionHours, values.EventRetentionHours, values.DeliveryRetentionHours} {
		if hours < 1 || hours > 8760 {
			return ErrRepositoryInvalidInput
		}
	}
	if values.LogLineLimit < 10 || values.LogLineLimit > 200 || values.DefaultAlertWindowSeconds < 60 || values.DefaultAlertWindowSeconds > 86400 || values.DefaultAlertPendingSeconds < 0 || values.DefaultAlertPendingSeconds > 3600 {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type ApplicationSettingsRecord struct {
	Version   int64
	UpdatedAt time.Time
	Values    ApplicationValues
}

func (record ApplicationSettingsRecord) Validate() error {
	if record.Version < 1 || record.Values.Validate() != nil || !validPolicyTime(record.UpdatedAt) {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type HistoryClass string

const (
	HistoryMetricSamples      HistoryClass = "metric_samples"
	HistoryHealth             HistoryClass = "health_history"
	HistoryEvents             HistoryClass = "infrastructure_events"
	HistoryTerminalDeliveries HistoryClass = "terminal_notification_deliveries"
)

func HistoryClasses() []HistoryClass {
	return []HistoryClass{HistoryMetricSamples, HistoryHealth, HistoryEvents, HistoryTerminalDeliveries}
}

func (class HistoryClass) Validate() error {
	switch class {
	case HistoryMetricSamples, HistoryHealth, HistoryEvents, HistoryTerminalDeliveries:
		return nil
	default:
		return ErrRepositoryInvalidInput
	}
}

func (values ApplicationValues) RetentionHours(class HistoryClass) int {
	switch class {
	case HistoryMetricSamples:
		return values.MetricRetentionHours
	case HistoryHealth:
		return values.HealthRetentionHours
	case HistoryEvents:
		return values.EventRetentionHours
	case HistoryTerminalDeliveries:
		return values.DeliveryRetentionHours
	default:
		return 0
	}
}

func ValidateHistoryClasses(classes []HistoryClass) error {
	if len(classes) < 1 || len(classes) > 4 {
		return ErrRepositoryInvalidInput
	}
	seen := map[HistoryClass]bool{}
	for _, class := range classes {
		if class.Validate() != nil || seen[class] {
			return ErrRepositoryInvalidInput
		}
		seen[class] = true
	}
	return nil
}

type HistoryCutoff struct {
	Name   HistoryClass `json:"name"`
	Cutoff time.Time    `json:"cutoff"`
}

type HistoryImpactClass struct {
	Name               HistoryClass `json:"name"`
	Cutoff             time.Time    `json:"cutoff"`
	EligibleRows       int          `json:"eligible_rows"`
	EligibleRowsCapped bool         `json:"eligible_rows_capped"`
}

type HistoryImpact struct {
	Classes             []HistoryImpactClass `json:"classes"`
	MaximumRows         int                  `json:"maximum_rows"`
	BackupRequired      bool                 `json:"backup_required"`
	ExpiryPolicyChanged bool                 `json:"expiry_policy_changed"`
}

func (impact HistoryImpact) Validate() error {
	if len(impact.Classes) > 4 || impact.MaximumRows != MaximumHistoryCleanupRows {
		return ErrRepositoryInvalidInput
	}
	seen := map[HistoryClass]bool{}
	for _, class := range impact.Classes {
		if class.Name.Validate() != nil || seen[class.Name] || !validPolicyTime(class.Cutoff) || class.EligibleRows < 0 || class.EligibleRows > HistoryImpactCountCap || class.EligibleRowsCapped != (class.EligibleRows == HistoryImpactCountCap) {
			return ErrRepositoryInvalidInput
		}
		seen[class.Name] = true
	}
	if impact.ExpiryPolicyChanged && (!impact.BackupRequired || len(impact.Classes) == 0) {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type AuditLifecyclePolicy struct {
	RetentionDays          int
	QuotaRows              int64
	ArchiveVerifiedThrough time.Time
}

func (policy AuditLifecyclePolicy) Validate() error {
	if policy.RetentionDays < MinimumAuditRetentionDays || policy.RetentionDays > 3650 || policy.QuotaRows < 1000 || policy.QuotaRows > 10000000 ||
		(!policy.ArchiveVerifiedThrough.IsZero() && !validPolicyTime(policy.ArchiveVerifiedThrough)) {
		return ErrRepositoryInvalidInput
	}
	return nil
}

type AuditPolicyStatus struct {
	RetentionDays          int        `json:"retention_days"`
	MinimumRetentionDays   int        `json:"minimum_retention_days"`
	QuotaRows              int64      `json:"quota_rows"`
	StoredRows             int64      `json:"stored_rows"`
	QuotaExceeded          bool       `json:"quota_exceeded"`
	ArchiveVerifiedThrough *time.Time `json:"archive_verified_through"`
}

func validPolicyTime(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC && value.UnixMicro() >= 0 && value.Nanosecond()%int(time.Microsecond) == 0
}
