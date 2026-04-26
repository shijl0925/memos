package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/usememos/memos/server/version"
)

// Profile is the configuration to start main server.
type Profile struct {
	// Mode can be "prod" or "dev" or "demo"
	Mode string `json:"mode"`
	// Port is the binding port for server
	Port int `json:"-"`
	// Data is the data directory
	Data string `json:"-"`
	// DSN points to where Memos stores its own data
	DSN string `json:"-"`
	// Driver is the database driver: sqlite3, mysql, or postgres
	Driver string `json:"-"`
	// Version is the current version of server
	Version string `json:"version"`
}

func (p *Profile) IsDev() bool {
	return p.Mode != "prod"
}

// GetProfile will return a profile for dev or prod.
func GetProfile() (*Profile, error) {
	profile := Profile{}
	if err := viper.Unmarshal(&profile); err != nil {
		return nil, err
	}

	if profile.Mode != "demo" && profile.Mode != "dev" && profile.Mode != "prod" {
		profile.Mode = "demo"
	}

	driver := viper.GetString("driver")
	if driver == "" {
		driver = "sqlite3"
	}
	profile.Driver = driver

	profile.Version = version.GetCurrentVersion(profile.Mode)

	if driver != "sqlite3" {
		profile.DSN = viper.GetString("dsn")
		return &profile, nil
	}

	// For SQLite, resolve the data directory relative to the current working
	// directory (project root) so the DB file is always predictably located
	// next to the project sources / binary.  An explicit --data flag still
	// takes precedence.
	dataDir := profile.Data
	if dataDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		dataDir = cwd
	} else if !filepath.IsAbs(dataDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		dataDir = filepath.Join(cwd, dataDir)
	}

	if _, err := os.Stat(dataDir); err != nil {
		return nil, fmt.Errorf("unable to access data folder %s: %w", dataDir, err)
	}

	profile.Data = dataDir
	profile.DSN = fmt.Sprintf("%s/memos_%s.db", dataDir, profile.Mode)

	return &profile, nil
}
