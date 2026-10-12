package postgres

import (
	"context"
	"testing"
	"time"
)

func validTestOptions() Options {
	var digestKey [authDigestKeyBytes]byte
	for index := range digestKey {
		digestKey[index] = byte(index + 1)
	}
	return Options{
		ConnectionString:          "postgresql://stackplore@127.0.0.1:5432/stackplore?sslmode=disable",
		Schema:                    "public",
		MinimumConnections:        2,
		MaximumConnections:        16,
		ConnectTimeout:            5 * time.Second,
		QueryTimeout:              10 * time.Second,
		MigrationTimeout:          time.Minute,
		HealthTimeout:             2 * time.Second,
		ShutdownTimeout:           5 * time.Second,
		MaximumConnectionLifetime: 30 * time.Minute,
		MaximumConnectionIdleTime: 5 * time.Minute,
		AuthDigestKey:             digestKey,
		AuthDigestKeyID:           "test_key_1",
	}
}

func TestOptionsRequireBoundedPoolAndTimeouts(t *testing.T) {
	t.Parallel()
	base := validTestOptions()
	if !validOptions(base) {
		t.Fatal("valid options were rejected")
	}
	tests := []struct {
		name   string
		change func(*Options)
	}{
		{"missing URL", func(value *Options) { value.ConnectionString = "" }},
		{"missing schema", func(value *Options) { value.Schema = "" }},
		{"unsafe schema", func(value *Options) { value.Schema = "bad-name" }},
		{"zero minimum", func(value *Options) { value.MinimumConnections = 0 }},
		{"minimum over maximum", func(value *Options) { value.MinimumConnections = value.MaximumConnections + 1 }},
		{"large maximum", func(value *Options) { value.MaximumConnections = 257 }},
		{"sole pool slot", func(value *Options) { value.MinimumConnections = 1; value.MaximumConnections = 1 }},
		{"short connect", func(value *Options) { value.ConnectTimeout = time.Millisecond }},
		{"short query", func(value *Options) { value.QueryTimeout = time.Millisecond }},
		{"short migration", func(value *Options) { value.MigrationTimeout = time.Second }},
		{"short health", func(value *Options) { value.HealthTimeout = time.Millisecond }},
		{"short shutdown", func(value *Options) { value.ShutdownTimeout = time.Millisecond }},
		{"short lifetime", func(value *Options) { value.MaximumConnectionLifetime = time.Second }},
		{"short idle", func(value *Options) { value.MaximumConnectionIdleTime = time.Millisecond }},
		{"zero digest key", func(value *Options) { value.AuthDigestKey = [authDigestKeyBytes]byte{} }},
		{"unsafe digest key ID", func(value *Options) { value.AuthDigestKeyID = "bad-key" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := base
			test.change(&candidate)
			if validOptions(candidate) {
				t.Fatal("invalid options were accepted")
			}
		})
	}
}

func TestBoundedContextKeepsShorterParentDeadline(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelParent()
	bounded, cancelBounded := boundedContext(parent, time.Second)
	defer cancelBounded()
	parentDeadline, _ := parent.Deadline()
	boundedDeadline, _ := bounded.Deadline()
	if !parentDeadline.Equal(boundedDeadline) {
		t.Fatalf("bounded deadline = %s; want parent %s", boundedDeadline, parentDeadline)
	}
}
