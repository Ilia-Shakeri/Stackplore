package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"
	"sort"
)

type ProviderKind string

const (
	ProviderDemo       ProviderKind = "demo"
	ProviderDocker     ProviderKind = "docker"
	ProviderKubernetes ProviderKind = "kubernetes"
)

func (kind ProviderKind) Validate() error {
	switch kind {
	case ProviderDemo, ProviderDocker, ProviderKubernetes:
		return nil
	default:
		return fmt.Errorf("unknown provider kind %q", kind)
	}
}

type SourceID string
type EnvironmentID string
type ResourceID string
type RelationshipID string
type EventID string
type AlertRuleID string
type AlertID string
type AuditEntryID string
type LogEntryID string
type MetricSampleID string
type UserID string
type RequestID string
type NativeResourceID string
type EvidenceID string

func (id SourceID) Validate() error { return validateToken("source ID", string(id), maxIDBytes) }
func (id EnvironmentID) Validate() error {
	return validateToken("environment ID", string(id), maxIDBytes)
}
func (id EventID) Validate() error     { return validateToken("event ID", string(id), maxIDBytes) }
func (id AlertRuleID) Validate() error { return validateToken("alert rule ID", string(id), maxIDBytes) }
func (id AlertID) Validate() error     { return validateToken("alert ID", string(id), maxIDBytes) }
func (id AuditEntryID) Validate() error {
	return validateToken("audit entry ID", string(id), maxIDBytes)
}
func (id LogEntryID) Validate() error {
	return validateToken("log entry ID", string(id), maxIDBytes)
}
func (id MetricSampleID) Validate() error {
	return validateToken("metric sample ID", string(id), maxIDBytes)
}
func (id UserID) Validate() error     { return validateToken("user ID", string(id), maxIDBytes) }
func (id RequestID) Validate() error  { return validateToken("request ID", string(id), maxIDBytes) }
func (id EvidenceID) Validate() error { return validateToken("evidence ID", string(id), maxIDBytes) }
func (id NativeResourceID) Validate() error {
	return validateNativeValue("native resource ID", string(id))
}

type ScopeKind string

const (
	ScopeGlobal    ScopeKind = "global"
	ScopeProject   ScopeKind = "project"
	ScopeNamespace ScopeKind = "namespace"
)

type Scope struct {
	Kind ScopeKind
	Name string
}

func (scope Scope) Validate() error {
	switch scope.Kind {
	case ScopeGlobal:
		if scope.Name != "" {
			return fmt.Errorf("global scope must not have a name")
		}
		return nil
	case ScopeProject, ScopeNamespace:
		return validateDisplayText("scope name", scope.Name, maxDisplayNameLength, false)
	default:
		return fmt.Errorf("unknown scope kind %q", scope.Kind)
	}
}

type ResourceKind string

func (kind ResourceKind) Validate() error {
	return validateNamespacedCode("resource kind", string(kind))
}

type ResourceIdentity struct {
	Provider      ProviderKind
	SourceID      SourceID
	EnvironmentID EnvironmentID
	Kind          ResourceKind
	Scope         Scope
	nativeID      NativeResourceID
}

func NewResourceIdentity(provider ProviderKind, sourceID SourceID, environmentID EnvironmentID, kind ResourceKind, scope Scope, nativeID NativeResourceID) (ResourceIdentity, error) {
	identity := ResourceIdentity{
		Provider: provider, SourceID: sourceID, EnvironmentID: environmentID, Kind: kind, Scope: scope, nativeID: nativeID,
	}
	if err := identity.Validate(); err != nil {
		return ResourceIdentity{}, err
	}
	return identity, nil
}

func (identity ResourceIdentity) NativeID() NativeResourceID {
	return identity.nativeID
}

func (identity ResourceIdentity) Validate() error {
	checks := []error{
		identity.Provider.Validate(),
		identity.SourceID.Validate(),
		identity.EnvironmentID.Validate(),
		identity.Kind.Validate(),
		identity.Scope.Validate(),
		identity.nativeID.Validate(),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func NewResourceID(identity ResourceIdentity) (ResourceID, error) {
	if err := identity.Validate(); err != nil {
		return "", fmt.Errorf("build resource ID: %w", err)
	}
	digest := sha256.New()
	writeIdentityPart(digest, "resource-id/v1")
	writeIdentityPart(digest, string(identity.Provider))
	writeIdentityPart(digest, string(identity.SourceID))
	writeIdentityPart(digest, string(identity.EnvironmentID))
	writeIdentityPart(digest, string(identity.Kind))
	writeIdentityPart(digest, string(identity.Scope.Kind))
	writeIdentityPart(digest, identity.Scope.Name)
	writeIdentityPart(digest, string(identity.nativeID))
	return ResourceID("r1_" + base64.RawURLEncoding.EncodeToString(digest.Sum(nil))), nil
}

func (id ResourceID) Validate() error {
	return validateDigestID("resource ID", string(id), "r1_")
}

type RelationshipIdentity struct {
	From      ResourceID
	To        ResourceID
	Kind      RelationshipKind
	Direction RelationshipDirection
}

func NewRelationshipID(identity RelationshipIdentity) (RelationshipID, error) {
	if err := identity.From.Validate(); err != nil {
		return "", err
	}
	if err := identity.To.Validate(); err != nil {
		return "", err
	}
	if identity.From == identity.To {
		return "", fmt.Errorf("relationship endpoints must differ")
	}
	if err := identity.Kind.Validate(); err != nil {
		return "", err
	}
	if err := identity.Direction.Validate(); err != nil {
		return "", err
	}
	fromID, toID := canonicalRelationshipEndpoints(identity.From, identity.To, identity.Direction)
	from, to := string(fromID), string(toID)
	digest := sha256.New()
	writeIdentityPart(digest, "relationship-id/v1")
	writeIdentityPart(digest, from)
	writeIdentityPart(digest, to)
	writeIdentityPart(digest, string(identity.Kind))
	writeIdentityPart(digest, string(identity.Direction))
	return RelationshipID("rel1_" + base64.RawURLEncoding.EncodeToString(digest.Sum(nil))), nil
}

func canonicalRelationshipEndpoints(from, to ResourceID, direction RelationshipDirection) (ResourceID, ResourceID) {
	if direction != RelationshipUndirected && direction != RelationshipBidirectional {
		return from, to
	}
	ends := []string{string(from), string(to)}
	sort.Strings(ends)
	return ResourceID(ends[0]), ResourceID(ends[1])
}

func (id RelationshipID) Validate() error {
	return validateDigestID("relationship ID", string(id), "rel1_")
}

func validateDigestID(name, value, prefix string) error {
	if len(value) <= len(prefix) || value[:len(prefix)] != prefix {
		return fmt.Errorf("%s has an invalid prefix", name)
	}
	digest, err := decodeCanonicalRawURLBase64(name+" digest", value[len(prefix):])
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("%s has an invalid digest", name)
	}
	return nil
}

func writeIdentityPart(digest hash.Hash, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}
