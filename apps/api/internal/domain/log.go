package domain

import (
	"fmt"
	"time"
	"unicode/utf8"
)

const (
	MaxLogTargets    = 32
	MaxLogLines      = MaxPageSize
	MaxLogBytes      = 8 << 20
	MaxLogLineBytes  = 64 << 10
	MaxLogLineLength = MaxLogLineBytes / 4
	MaxLogLookback   = 7 * 24 * time.Hour
	MaxStreamBuffer  = 1024
)

type LogStreamKind string

const (
	LogStreamStdout  LogStreamKind = "stdout"
	LogStreamStderr  LogStreamKind = "stderr"
	LogStreamSystem  LogStreamKind = "system"
	LogStreamUnknown LogStreamKind = "unknown"
)

func (kind LogStreamKind) Validate() error {
	switch kind {
	case LogStreamStdout, LogStreamStderr, LogStreamSystem, LogStreamUnknown:
		return nil
	default:
		return fmt.Errorf("unknown log stream %q", kind)
	}
}

type LogTarget struct {
	Resource  ResourceRef
	Container string
	Previous  bool
}

func (target LogTarget) Validate() error {
	if err := target.Resource.Validate(); err != nil {
		return err
	}
	return validateDisplayText("container name", target.Container, maxDisplayNameLength, true)
}

func (target LogTarget) key() string {
	return string(target.Resource.ID) + "\x00" + target.Container
}

type LogRecord struct {
	ID         LogEntryID
	ResourceID ResourceID
	Container  string
	Stream     LogStreamKind
	Sequence   uint64
	Severity   LogSeverity
	Timestamp  time.Time
	Message    string
	Redacted   bool
	Truncated  bool
	Synthetic  bool
}

func (record LogRecord) Validate() error {
	if err := record.ID.Validate(); err != nil {
		return err
	}
	if err := record.ResourceID.Validate(); err != nil {
		return err
	}
	if err := validateDisplayText("container name", record.Container, maxDisplayNameLength, true); err != nil {
		return err
	}
	if err := record.Stream.Validate(); err != nil {
		return err
	}
	if err := record.Severity.Validate(); err != nil {
		return err
	}
	if err := validateUTCTime("log timestamp", record.Timestamp); err != nil {
		return err
	}
	if !utf8.ValidString(record.Message) {
		return fmt.Errorf("log message must be valid UTF-8")
	}
	if len(record.Message) > MaxLogLineBytes {
		return fmt.Errorf("log message exceeds %d bytes", MaxLogLineBytes)
	}
	if utf8.RuneCountInString(record.Message) > MaxLogLineLength {
		return fmt.Errorf("log message exceeds %d characters", MaxLogLineLength)
	}
	for _, char := range record.Message {
		if isUnsafeDisplayRune(char) {
			return fmt.Errorf("log message contains an unsafe control character")
		}
	}
	if record.Sequence > 1<<53-1 {
		return fmt.Errorf("log sequence exceeds the safe integer range")
	}
	return nil
}

func (record LogRecord) ValidateForTail(query LogTailQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	if len(record.Message) > query.MaxLineBytes {
		return fmt.Errorf("log tail record exceeds requested line size")
	}
	if record.Timestamp.Before(query.Since) {
		return fmt.Errorf("log tail record predates requested replay window")
	}
	for _, target := range query.Targets {
		if record.ResourceID == target.Resource.ID && record.Container == target.Container {
			return nil
		}
	}
	return fmt.Errorf("log tail record belongs to an unrequested target")
}

type LogSeverity string

const (
	LogSeverityTrace    LogSeverity = "trace"
	LogSeverityDebug    LogSeverity = "debug"
	LogSeverityInfo     LogSeverity = "info"
	LogSeverityWarning  LogSeverity = "warning"
	LogSeverityError    LogSeverity = "error"
	LogSeverityCritical LogSeverity = "critical"
	LogSeverityUnknown  LogSeverity = "unknown"
)

func (severity LogSeverity) Validate() error {
	switch severity {
	case LogSeverityTrace, LogSeverityDebug, LogSeverityInfo, LogSeverityWarning, LogSeverityError, LogSeverityCritical, LogSeverityUnknown:
		return nil
	default:
		return fmt.Errorf("unknown log severity %q", severity)
	}
}

type LogReadQuery struct {
	Source    ProviderRef
	Targets   []LogTarget
	Range     TimeRange
	LineLimit int
	ByteLimit int
	Cursor    PageCursor
}

func (query LogReadQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := validateLogTargets(query.Source, query.Targets); err != nil {
		return err
	}
	if err := query.Range.ValidateLookback(MaxLogLookback); err != nil {
		return err
	}
	if query.LineLimit < 1 || query.LineLimit > MaxLogLines {
		return fmt.Errorf("log line limit must be between 1 and %d", MaxLogLines)
	}
	if query.ByteLimit < 1 || query.ByteLimit > MaxLogBytes {
		return fmt.Errorf("log byte limit must be between 1 and %d", MaxLogBytes)
	}
	return query.Cursor.Validate()
}

type LogPage struct {
	Status  ObservationStatus
	Records []LogRecord
	Page    PageInfo
}

func (page LogPage) ValidateFor(query LogReadQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := page.Status.Validate(); err != nil {
		return err
	}
	if err := page.Page.ValidateFor(PageRequest{Limit: query.LineLimit, Cursor: query.Cursor}); err != nil {
		return err
	}
	if len(page.Records) > query.LineLimit {
		return fmt.Errorf("log page exceeds requested line limit")
	}
	if err := validatePageAvailability(page.Status, page.Page, len(page.Records)); err != nil {
		return err
	}
	targets := make(map[string]struct{}, len(query.Targets))
	for _, target := range query.Targets {
		targets[string(target.Resource.ID)+"\x00"+target.Container] = struct{}{}
	}
	bytesUsed := 0
	seenIDs := make(map[LogEntryID]struct{}, len(page.Records))
	var previousSequence uint64
	for _, record := range page.Records {
		if err := record.Validate(); err != nil {
			return err
		}
		if _, exists := targets[string(record.ResourceID)+"\x00"+record.Container]; !exists {
			return fmt.Errorf("log page contains an unrequested target")
		}
		if record.Timestamp.Before(query.Range.Start) || !record.Timestamp.Before(query.Range.End) {
			return fmt.Errorf("log page contains a record outside the requested range")
		}
		if _, exists := seenIDs[record.ID]; exists {
			return fmt.Errorf("log page contains a duplicate entry ID")
		}
		seenIDs[record.ID] = struct{}{}
		if len(seenIDs) > 1 && record.Sequence <= previousSequence {
			return fmt.Errorf("log page sequence must increase")
		}
		previousSequence = record.Sequence
		bytesUsed += len(record.Message)
		if bytesUsed > query.ByteLimit {
			return fmt.Errorf("log page exceeds requested byte limit")
		}
	}
	return nil
}

type LogTailQuery struct {
	Source  ProviderRef
	Targets []LogTarget
	Since   time.Time
	// RequestedAt is a trusted service timestamp, never client input.
	RequestedAt  time.Time
	BufferSize   int
	MaxLineBytes int
}

type LogTailValidator struct {
	query        LogTailQuery
	hasRecord    bool
	lastSequence uint64
}

func NewLogTailValidator(query LogTailQuery) (*LogTailValidator, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	query.Targets = append([]LogTarget(nil), query.Targets...)
	return &LogTailValidator{query: query}, nil
}

func (validator *LogTailValidator) Validate(record LogRecord) error {
	if validator == nil {
		return fmt.Errorf("log tail validator must not be nil")
	}
	if err := record.ValidateForTail(validator.query); err != nil {
		return err
	}
	if validator.hasRecord && record.Sequence <= validator.lastSequence {
		return fmt.Errorf("log tail sequence must increase")
	}
	validator.hasRecord = true
	validator.lastSequence = record.Sequence
	return nil
}

func (query LogTailQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := validateLogTargets(query.Source, query.Targets); err != nil {
		return err
	}
	if err := validateReplayWindow("log tail", query.Since, query.RequestedAt, MaxLogLookback); err != nil {
		return err
	}
	if query.BufferSize < 1 || query.BufferSize > MaxStreamBuffer {
		return fmt.Errorf("log tail buffer must be between 1 and %d", MaxStreamBuffer)
	}
	if query.MaxLineBytes < 1 || query.MaxLineBytes > MaxLogLineBytes {
		return fmt.Errorf("log tail line size must be between 1 and %d", MaxLogLineBytes)
	}
	return nil
}

func validateLogTargets(source ProviderRef, targets []LogTarget) error {
	if len(targets) == 0 || len(targets) > MaxLogTargets {
		return fmt.Errorf("log query must have between 1 and %d targets", MaxLogTargets)
	}
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if err := target.Validate(); err != nil {
			return err
		}
		if !source.Matches(target.Resource) {
			return fmt.Errorf("log target does not belong to query source")
		}
		key := target.key()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("log query has a duplicate target")
		}
		seen[key] = struct{}{}
	}
	return nil
}
