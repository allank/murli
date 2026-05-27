package murli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Profile holds a named set of flag values, stored as strings.
type Profile struct {
	Flags map[string]string `json:"flags"`
}

// ProfileStore is the on-disk representation of all saved profiles for a tool.
type ProfileStore struct {
	Default  string             `json:"default,omitempty"`
	Profiles map[string]Profile `json:"profiles"`
}

// ProfilePath returns the path to the profiles file for the named tool.
// Resolves to ~/.{toolName}/profiles.json.
func ProfilePath(toolName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "."+toolName, "profiles.json")
	}
	return filepath.Join(home, "."+toolName, "profiles.json")
}

// LoadProfileStore reads the profile store from disk.
// Returns an empty store (not an error) if the file does not exist.
func LoadProfileStore(toolName string) (*ProfileStore, error) {
	path := ProfilePath(toolName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &ProfileStore{Profiles: make(map[string]Profile)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("murli: read profile store: %w", err)
	}
	var s ProfileStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("murli: parse profile store: %w", err)
	}
	if s.Profiles == nil {
		s.Profiles = make(map[string]Profile)
	}
	return &s, nil
}

// Save writes the store back to disk, creating the directory if needed.
func (s *ProfileStore) Save(toolName string) error {
	path := ProfilePath(toolName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("murli: create profile dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("murli: marshal profile store: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("murli: write profile store: %w", err)
	}
	return nil
}

// Get returns the named profile. Returns a zero Profile and false if not found.
func (s *ProfileStore) Get(name string) (Profile, bool) {
	p, ok := s.Profiles[name]
	return p, ok
}

// Set adds or replaces a named profile.
func (s *ProfileStore) Set(name string, p Profile) {
	if s.Profiles == nil {
		s.Profiles = make(map[string]Profile)
	}
	s.Profiles[name] = p
}

// Delete removes a profile. Clears Default if the deleted profile was the default.
// No-op if the profile does not exist.
func (s *ProfileStore) Delete(name string) {
	delete(s.Profiles, name)
	if s.Default == name {
		s.Default = ""
	}
}

// SetDefault marks a profile as the default.
// Returns an error if the named profile does not exist.
func (s *ProfileStore) SetDefault(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	s.Default = name
	return nil
}

// Names returns all profile names in sorted order.
func (s *ProfileStore) Names() []string {
	names := make([]string, 0, len(s.Profiles))
	for name := range s.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
