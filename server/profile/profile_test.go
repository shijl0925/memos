package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestGetProfile_SQLite_DefaultsToWorkingDirectory verifies that when no
// --data flag is provided the SQLite DSN points to the current working
// directory (project root), not to the binary directory.
func TestGetProfile_SQLite_DefaultsToWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	viper.SetDefault("mode", "demo")
	viper.SetDefault("port", 8081)
	viper.SetDefault("driver", "sqlite3")

	prof, err := GetProfile()
	if err != nil {
		t.Fatalf("GetProfile() returned error: %v", err)
	}

	wantDSN := fmt.Sprintf("%s/memos_%s.db", cwd, prof.Mode)
	if prof.DSN != wantDSN {
		t.Errorf("DSN = %q, want %q", prof.DSN, wantDSN)
	}
	if prof.Data != cwd {
		t.Errorf("Data = %q, want %q (cwd)", prof.Data, cwd)
	}
}

// TestGetProfile_SQLite_ExplicitDataAbsolute verifies that an explicit
// absolute --data path is respected.
func TestGetProfile_SQLite_ExplicitDataAbsolute(t *testing.T) {
	dir := t.TempDir()

	viper.Reset()
	viper.SetDefault("mode", "demo")
	viper.SetDefault("port", 8081)
	viper.SetDefault("driver", "sqlite3")
	viper.Set("data", dir)

	prof, err := GetProfile()
	if err != nil {
		t.Fatalf("GetProfile() returned error: %v", err)
	}

	wantDSN := fmt.Sprintf("%s/memos_%s.db", dir, prof.Mode)
	if prof.DSN != wantDSN {
		t.Errorf("DSN = %q, want %q", prof.DSN, wantDSN)
	}
	if prof.Data != dir {
		t.Errorf("Data = %q, want %q", prof.Data, dir)
	}
}

// TestGetProfile_SQLite_ExplicitDataRelative verifies that a relative --data
// path is resolved against the current working directory.
func TestGetProfile_SQLite_ExplicitDataRelative(t *testing.T) {
	// Create a subdirectory inside the working directory and use a
	// relative reference to it.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sub := t.TempDir()
	// Make the path relative (sub is absolute; strip cwd prefix for a
	// relative reference).
	rel, err := filepath.Rel(cwd, sub)
	if err != nil {
		t.Skip("temp dir not relative to cwd, skipping")
	}
	if strings.HasPrefix(rel, "..") {
		t.Skip("temp dir outside cwd, skipping")
	}

	viper.Reset()
	viper.SetDefault("mode", "dev")
	viper.SetDefault("port", 8081)
	viper.SetDefault("driver", "sqlite3")
	viper.Set("data", rel)

	prof, err := GetProfile()
	if err != nil {
		t.Fatalf("GetProfile() returned error: %v", err)
	}

	wantData := filepath.Join(cwd, rel)
	wantDSN := fmt.Sprintf("%s/memos_%s.db", wantData, prof.Mode)
	if prof.Data != wantData {
		t.Errorf("Data = %q, want %q", prof.Data, wantData)
	}
	if prof.DSN != wantDSN {
		t.Errorf("DSN = %q, want %q", prof.DSN, wantDSN)
	}
}

// TestGetProfile_NonSQLite verifies that non-SQLite drivers use the dsn flag
// directly and do not touch the data directory.
func TestGetProfile_NonSQLite(t *testing.T) {
	const dsn = "root:password@tcp(localhost:3306)/memos"

	viper.Reset()
	viper.SetDefault("mode", "prod")
	viper.SetDefault("port", 8081)
	viper.Set("driver", "mysql")
	viper.Set("dsn", dsn)

	prof, err := GetProfile()
	if err != nil {
		t.Fatalf("GetProfile() returned error: %v", err)
	}

	if prof.DSN != dsn {
		t.Errorf("DSN = %q, want %q", prof.DSN, dsn)
	}
	if prof.Data != "" {
		t.Errorf("Data = %q, want empty string for non-SQLite", prof.Data)
	}
}
