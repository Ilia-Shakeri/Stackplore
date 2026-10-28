package config

import (
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	auditRetentionDaysKey          = prefix + "AUDIT_RETENTION_DAYS"
	auditQuotaRowsKey              = prefix + "AUDIT_QUOTA_ROWS"
	auditArchiveVerifiedThroughKey = prefix + "AUDIT_ARCHIVE_VERIFIED_THROUGH"
	ApplicationVersion             = "2.0.0-alpha.4"
)

var auditPolicyKeys = []string{auditRetentionDaysKey, auditQuotaRowsKey, auditArchiveVerifiedThroughKey}

type AuditPolicyConfig struct {
	AuditRetentionDays          int
	AuditQuotaRows              int64
	AuditArchiveVerifiedThrough time.Time
}

func loadAuditPolicy(getter Getter, result *Config) error {
	result.AuditRetentionDays = 365
	result.AuditQuotaRows = 1000000
	for _, field := range []struct {
		key              string
		minimum, maximum int64
		assign           func(int64)
	}{
		{auditRetentionDaysKey, 90, 3650, func(value int64) { result.AuditRetentionDays = int(value) }},
		{auditQuotaRowsKey, 1000, 10000000, func(value int64) { result.AuditQuotaRows = value }},
	} {
		value, present, err := readValue(getter, field.key)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || strconv.FormatInt(parsed, 10) != value || parsed < field.minimum || parsed > field.maximum {
			return invalid(field.key, "is outside its safe range")
		}
		field.assign(parsed)
	}
	if value, present, err := readValue(getter, auditArchiveVerifiedThroughKey); err != nil {
		return err
	} else if present {
		at, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || at.Location() != time.UTC || at.UnixMicro() < 0 || at.Nanosecond()%int(time.Microsecond) != 0 || len(value) > 35 || at.After(time.Now().UTC()) {
			return invalid(auditArchiveVerifiedThroughKey, "must name an already verified UTC archive cutoff")
		}
		result.AuditArchiveVerifiedThrough = at
	}
	return nil
}

func (settings AuditPolicyConfig) LifecyclePolicy() domain.AuditLifecyclePolicy {
	days, quota := settings.AuditRetentionDays, settings.AuditQuotaRows
	if days == 0 {
		days = 365
	}
	if quota == 0 {
		quota = 1000000
	}
	return domain.AuditLifecyclePolicy{RetentionDays: days, QuotaRows: quota, ArchiveVerifiedThrough: settings.AuditArchiveVerifiedThrough}
}

func validateAuditPolicy(settings Config) error {
	if settings.AuditPolicyConfig.LifecyclePolicy().Validate() != nil {
		return invalid(auditRetentionDaysKey, "violates protected audit policy")
	}
	return nil
}
