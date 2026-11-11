// Package state reads and writes the Go core's own JSON state file under
// CoreData(), and reads legacy marker files written by the Python installer.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

const currentSchema = "core/v1"

// State is the core's own persistent state, stored in CoreData()/state.json.
type State struct {
	Schema string `json:"schema"`
}

// ErrSchemaMismatch is returned when the state file has an unrecognised schema.
type ErrSchemaMismatch struct {
	Got string
}

func (e *ErrSchemaMismatch) Error() string {
	return fmt.Sprintf("state schema mismatch: got %q, want %q", e.Got, currentSchema)
}

// Is makes errors.Is work with schema mismatch checks.
func (e *ErrSchemaMismatch) Is(target error) bool {
	_, ok := target.(*ErrSchemaMismatch)
	return ok
}

// ErrSchemaMismatchSentinel is a zero-value sentinel for use with errors.Is.
var ErrSchemaMismatchSentinel = &ErrSchemaMismatch{}

// Read reads the core state from CoreData()/state.json.
// If the file does not exist, a default State with the current schema is returned.
func Read(root paths.Root) (*State, error) {
	data, err := os.ReadFile(stateFile(root))
	if errors.Is(err, os.ErrNotExist) {
		return &State{Schema: currentSchema}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	if s.Schema != currentSchema {
		return nil, &ErrSchemaMismatch{Got: s.Schema}
	}
	return &s, nil
}

// Write atomically writes s to CoreData()/state.json.
func Write(root paths.Root, s *State) error {
	s.Schema = currentSchema
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	dir := root.CoreData()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "state-*.tmp.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful Rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, stateFile(root)); err != nil {
		return fmt.Errorf("renaming state file: %w", err)
	}
	return nil
}

// ReadGPUMarker reads installer_scripts/.gpu and returns its trimmed content.
// Returns ("", error) if the file does not exist or cannot be read.
func ReadGPUMarker(root paths.Root) (string, error) {
	data, err := os.ReadFile(root.MarkerFile(".gpu"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ReadPreflight reports whether installer_scripts/.preflight_ok exists.
func ReadPreflight(root paths.Root) (bool, error) {
	_, err := os.Stat(root.MarkerFile(".preflight_ok"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func stateFile(root paths.Root) string {
	return filepath.Join(root.CoreData(), "state.json")
}
