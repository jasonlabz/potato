package migrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonlabz/potato/internal/log"
	potatolog "github.com/jasonlabz/potato/log"
)

func testLogger() log.Logger {
	return potatolog.GetLogger()
}

func TestLoadMigrationFilesUsesDirectoryTypeAndFilenameVersion(t *testing.T) {
	ddlDir := t.TempDir()
	writeMigrationTestFile(t, ddlDir, "20260903_002_add_column.sql", "-- ALTER TABLE demo ADD COLUMN name TEXT;\n")
	writeMigrationTestFile(t, ddlDir, "20260903_001_add_table.sql", "-- CREATE TABLE demo (id BIGINT);\n")

	ddlFiles := loadMigrationFiles(context.Background(), testLogger(), ddlDir, MigrationTypeDDL)
	if len(ddlFiles) != 2 {
		t.Fatalf("expected all files to use the directory default, got %d", len(ddlFiles))
	}
	if ddlFiles[0].kind != MigrationTypeDDL {
		t.Fatalf("expected default type ddl, got %q", ddlFiles[0].kind)
	}
	if ddlFiles[0].version != "20260903_001" {
		t.Fatalf("expected filename version, got %q", ddlFiles[0].version)
	}

	seedDir := t.TempDir()
	writeMigrationTestFile(t, seedDir, "20260903_002_seed_roles.sql", "-- SELECT 2;\n")
	writeMigrationTestFile(t, seedDir, "20260903_001_seed_users.sql", "-- SELECT 1;\n")

	seedFiles := loadMigrationFiles(context.Background(), testLogger(), seedDir, MigrationTypeSeed)
	if len(seedFiles) != 2 {
		t.Fatalf("expected all files to use the directory default, got %d", len(seedFiles))
	}
	if seedFiles[0].kind != MigrationTypeSeed {
		t.Fatalf("expected default type seed, got %q", seedFiles[0].kind)
	}
	if seedFiles[0].version != "20260903_001" {
		t.Fatalf("expected seed filename version, got %q", seedFiles[0].version)
	}
}

func TestLoadMigrationFilesSkipsNonstandardSeedNames(t *testing.T) {
	dir := t.TempDir()
	writeMigrationTestFile(t, dir, "legacy_seed.sql", "-- SELECT 1;\n")
	writeMigrationTestFile(t, dir, "20260903_001.sql", "-- SELECT 2;\n")
	writeMigrationTestFile(t, dir, "20260903_abc_invalid.sql", "-- SELECT 3;\n")
	writeMigrationTestFile(t, dir, "20260903_001_valid_seed.sql", "-- SELECT 4;\n")

	files := loadMigrationFiles(context.Background(), testLogger(), dir, MigrationTypeSeed)
	if len(files) != 1 {
		t.Fatalf("expected only the standard seed file, got %d", len(files))
	}
	if files[0].name != "20260903_001_valid_seed.sql" {
		t.Fatalf("expected standard seed file, got %q", files[0].name)
	}
}

func TestResolveVersionKeepsDDLHeaderVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "00000000_000_pg_baseline.sql")
	writeMigrationTestFile(t, dir, "00000000_000_pg_baseline.sql", "-- @version 20260902_003\n-- CREATE TABLE demo (id BIGINT);\n")

	if got := resolveVersion(path, filepath.Base(path)); got != "20260902_003" {
		t.Fatalf("expected DDL header version, got %q", got)
	}
}

func TestResolveVersionUsesHeaderForSeedBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "00000000_000_baseline.sql")
	writeMigrationTestFile(t, dir, "00000000_000_baseline.sql", "-- @version 20260903_001\n-- INSERT INTO demo ...;\n")

	if got := resolveVersion(path, filepath.Base(path)); got != "20260903_001" {
		t.Fatalf("expected seed baseline header version, got %q", got)
	}
}

func TestResolveVersionRequiresBaselineHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "00000000_000_baseline.sql")
	writeMigrationTestFile(t, dir, "00000000_000_baseline.sql", "-- CREATE TABLE demo (id BIGINT);\n")

	if got := resolveVersion(path, filepath.Base(path)); got != "" {
		t.Fatalf("expected baseline without header to be rejected, got %q", got)
	}
}

func TestResolveVersionFallsBackToFilenameForMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260903_001_add_column.sql")
	writeMigrationTestFile(t, dir, filepath.Base(path), "ALTER TABLE demo ADD COLUMN name TEXT;\n")

	if got := resolveVersion(path, filepath.Base(path)); got != "20260903_001" {
		t.Fatalf("expected filename version fallback, got %q", got)
	}
}

func writeMigrationTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write test migration %s: %v", name, err)
	}
}
