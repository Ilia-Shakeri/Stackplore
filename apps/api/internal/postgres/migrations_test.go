package postgres

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsAcceptsOrderedImmutableFiles(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{
		"000002_more.sql": {Data: []byte("CREATE TABLE second_table (id bigint PRIMARY KEY);")},
		"000001_base.sql": {Data: []byte("CREATE TABLE first_table (id bigint PRIMARY KEY);")},
	}
	loaded, err := loadMigrations(files)
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(loaded) != 2 || loaded[0].version != 1 || loaded[1].version != 2 || loaded[0].filename != "000001_base.sql" {
		t.Fatalf("loadMigrations() = %#v; want ordered versions", loaded)
	}
	if loaded[0].checksum == loaded[1].checksum {
		t.Fatal("distinct migration content produced one checksum")
	}
}

func TestLoadMigrationsRejectsUnsafeHistory(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files fstest.MapFS
	}{
		{"empty", fstest.MapFS{}},
		{"gap", fstest.MapFS{"000002_gap.sql": {Data: []byte("SELECT 1;")}}},
		{"bad name", fstest.MapFS{"1_bad.sql": {Data: []byte("SELECT 1;")}}},
		{"uppercase", fstest.MapFS{"000001_BAD.sql": {Data: []byte("SELECT 1;")}}},
		{"empty file", fstest.MapFS{"000001_empty.sql": {Data: nil}}},
		{"nul", fstest.MapFS{"000001_nul.sql": {Data: []byte{'S', 0}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadMigrations(test.files)
			if !errors.Is(err, ErrMigrationRejected) {
				t.Fatalf("loadMigrations() error = %v; want migration rejection", err)
			}
		})
	}
}

func TestMigrationVersionRejectsNonCanonicalNames(t *testing.T) {
	t.Parallel()
	for _, filename := range []string{"", "../000001_x.sql", "000000_zero.sql", "000001_.sql", "000001_x.SQL", "000001_x-y.sql"} {
		if _, ok := migrationVersion(filename); ok {
			t.Errorf("migrationVersion(%q) accepted", filename)
		}
	}
}

func TestMigrationHistoryMatchRequiresExactLedger(t *testing.T) {
	t.Parallel()
	loaded, err := loadMigrations(fstest.MapFS{
		"000001_base.sql": {Data: []byte("CREATE TABLE first_table (id bigint PRIMARY KEY);")},
	})
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	exact := []appliedMigration{{version: 1, filename: loaded[0].filename, checksum: loaded[0].checksum[:]}}
	if !migrationHistoryMatches(exact, loaded) {
		t.Fatal("exact ledger was rejected")
	}
	changed := append([]byte(nil), exact[0].checksum...)
	changed[0] ^= 0xff
	if migrationHistoryMatches([]appliedMigration{{version: 1, filename: exact[0].filename, checksum: changed}}, loaded) {
		t.Fatal("changed checksum was accepted")
	}
	if migrationHistoryMatches(nil, loaded) {
		t.Fatal("missing ledger was accepted")
	}
}
