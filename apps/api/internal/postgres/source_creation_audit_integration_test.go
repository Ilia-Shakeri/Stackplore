package postgres

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"testing"
)

func TestOperatorSourceCreationIsAuditedOnceAndAtomically(t *testing.T) {
	f := newObservationFixture(t)
	configuration, err := f.store.GetSourceConfiguration(f.ctx, f.token.Source.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.EnsureSourceConfigurations(f.ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.store.pool.QueryRow(f.ctx, `SELECT count(*) FROM audit_entries WHERE action='source.create' AND target_id=$1 AND actor_kind='system' AND actor_user_id IS NULL`, string(configuration.Source.ID)).Scan(&count); err != nil || count != 1 {
		t.Fatal("source provisioning audit missing or duplicated")
	}
	_, err = f.store.pool.Exec(f.ctx, `CREATE FUNCTION reject_source_creation_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='source.create' THEN RAISE EXCEPTION 'disposable audit rejection'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_source_creation_audit BEFORE INSERT ON audit_entries FOR EACH ROW EXECUTE FUNCTION reject_source_creation_audit()`)
	if err != nil {
		t.Fatal(err)
	}
	configuration.Source.ID = "source_atomic_audit"
	if err = f.store.EnsureSourceConfigurations(f.ctx, []domain.SourceConfiguration{configuration}); err == nil {
		t.Fatal("source created without durable audit")
	}
	if err = f.store.pool.QueryRow(f.ctx, `SELECT count(*) FROM infrastructure_sources WHERE id='source_atomic_audit'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("audit rejection left a source behind")
	}
}
