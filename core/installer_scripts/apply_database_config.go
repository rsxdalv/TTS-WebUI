package installer_scripts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ApplyDatabaseConfig ports installer_scripts/js/applyDatabaseConfig.js.
//
// It creates <root>/data/sqlite, then, only when installer_scripts/.db_version
// already exists and differs from "1", runs the upgrade. The original upgrade
// calls createDB(), which is not defined, so it always throws a ReferenceError
// before writing .db_version. The port keeps that: the error is returned, the
// marker is not written, and the caller (initializeApp.js:292-297 in the
// original) logs it and carries on. exitCode is 0 on success and 1 on error.
func ApplyDatabaseConfig(ctx context.Context, s *Step) (exitCode int, err error) {
	if err := applyDatabaseConfig(s); err != nil {
		return 1, err
	}
	return 0, nil
}

func applyDatabaseConfig(s *Step) error {
	const dbVersion = "1"
	location := filepath.Join(s.Root, "data", "sqlite")
	database := "webui.db"

	if _, err := os.Stat(location); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		s.Log("Creating SQLite database directory...")
		if err := os.MkdirAll(location, 0o777); err != nil {
			return err
		}
		s.Log("Successfully created SQLite database directory")
	} else {
		s.Log("SQLite database directory already exists, skipping...")
	}

	dbVersionFile := filepath.Join(s.Root, "installer_scripts", ".db_version")
	if _, err := os.Stat(dbVersionFile); err != nil {
		// JS fs.existsSync: false for any error. A fresh install ends here.
		return nil
	}
	raw, err := os.ReadFile(dbVersionFile)
	if err != nil {
		return err
	}
	version := string(raw) // raw compare, no trim, as the original
	if version == dbVersion {
		s.Log(fmt.Sprintf("Database is already up to date with version=%s, skipping...", version))
		return nil
	}
	s.Log(fmt.Sprintf("Database is not up to date, current version=%s, version=%s, upgrading...", version, dbVersion))

	// withDatabase
	s.Log("Using SQLite database...")
	s.Log("Using SQLite database at " + filepath.Join(location, database))
	// Original: `await createDB()`, which is undefined. See the contract's
	// known-problems table: keep the failure, do not write .db_version.
	return errors.New("createDB is not defined")
}
