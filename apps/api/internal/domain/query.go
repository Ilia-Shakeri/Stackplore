package domain

import (
	"fmt"
	"time"
)

const (
	MaxPageSize        = 200
	MaxPageCursorBytes = 4096
	MaxFilterValues    = 64
	MaxSearchLength    = 200
)

type PageCursor string

func (cursor PageCursor) Validate() error {
	if cursor == "" {
		return nil
	}
	if len(cursor) > MaxPageCursorBytes {
		return fmt.Errorf("page cursor exceeds %d bytes", MaxPageCursorBytes)
	}
	if _, err := decodeCanonicalRawURLBase64("page cursor", string(cursor)); err != nil {
		return err
	}
	return nil
}

type PageRequest struct {
	Limit  int
	Cursor PageCursor
}

func (request PageRequest) Validate() error {
	if request.Limit < 1 || request.Limit > MaxPageSize {
		return fmt.Errorf("page limit must be between 1 and %d", MaxPageSize)
	}
	return request.Cursor.Validate()
}

type PageInfo struct {
	Limit      int
	NextCursor PageCursor
	HasMore    bool
}

func (info PageInfo) Validate() error {
	if info.Limit < 1 || info.Limit > MaxPageSize {
		return fmt.Errorf("applied page limit must be between 1 and %d", MaxPageSize)
	}
	if err := info.NextCursor.Validate(); err != nil {
		return err
	}
	if info.HasMore && info.NextCursor == "" {
		return fmt.Errorf("next cursor is required when more items exist")
	}
	if !info.HasMore && info.NextCursor != "" {
		return fmt.Errorf("next cursor must be empty on the last page")
	}
	return nil
}

func (info PageInfo) ValidateFor(request PageRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := info.Validate(); err != nil {
		return err
	}
	if info.Limit != request.Limit {
		return fmt.Errorf("page limit does not match request")
	}
	if info.HasMore && info.NextCursor == request.Cursor {
		return fmt.Errorf("next cursor must differ from the request cursor")
	}
	return nil
}

func validatePageAvailability(status ObservationStatus, info PageInfo, itemCount int) error {
	if status.AllowsData() {
		return nil
	}
	if itemCount != 0 {
		return fmt.Errorf("unavailable page must not contain data")
	}
	if info.HasMore {
		return fmt.Errorf("unavailable page must not advertise more data")
	}
	return nil
}

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

func (direction SortDirection) Validate() error {
	switch direction {
	case SortAscending, SortDescending:
		return nil
	default:
		return fmt.Errorf("unknown sort direction %q", direction)
	}
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

func (value TimeRange) Validate() error {
	if err := validateUTCTime("range start", value.Start); err != nil {
		return err
	}
	if err := validateUTCTime("range end", value.End); err != nil {
		return err
	}
	if !value.Start.Before(value.End) {
		return fmt.Errorf("range start must be before range end")
	}
	return nil
}

func (value TimeRange) ValidateLookback(maximum time.Duration) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if maximum <= 0 {
		return fmt.Errorf("maximum lookback must be positive")
	}
	if value.End.Sub(value.Start) > maximum {
		return fmt.Errorf("time range exceeds maximum lookback")
	}
	return nil
}

func validateReplayWindow(name string, since time.Time, requestedAt time.Time, maximum time.Duration) error {
	if err := validateUTCTime(name+" start", since); err != nil {
		return err
	}
	if err := validateUTCTime(name+" request time", requestedAt); err != nil {
		return err
	}
	if maximum <= 0 {
		return fmt.Errorf("%s maximum lookback must be positive", name)
	}
	if since.After(requestedAt) {
		return fmt.Errorf("%s start cannot be after request time", name)
	}
	if requestedAt.Sub(since) > maximum {
		return fmt.Errorf("%s exceeds maximum lookback", name)
	}
	return nil
}
