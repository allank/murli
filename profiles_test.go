package murli_test

import (
	"path/filepath"
	"testing"

	"github.com/murli-cli/murli-go"
)

func TestLoadProfileStoreReturnsEmptyWhenMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := murli.LoadProfileStore("notexist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if len(store.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %v", store.Profiles)
	}
	if store.Default != "" {
		t.Errorf("expected empty default, got %q", store.Default)
	}
}

func TestProfileStoreSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := &murli.ProfileStore{
		Profiles: map[string]murli.Profile{
			"prod": {Flags: map[string]string{"region": "us-east-1", "token": "abc"}},
		},
	}
	if err := store.Save("mytool"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := murli.LoadProfileStore("mytool")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, ok := loaded.Get("prod")
	if !ok {
		t.Fatal("prod profile not found after load")
	}
	if p.Flags["region"] != "us-east-1" {
		t.Errorf("expected region=us-east-1, got %q", p.Flags["region"])
	}
	if p.Flags["token"] != "abc" {
		t.Errorf("expected token=abc, got %q", p.Flags["token"])
	}
}

func TestProfileStoreSetGet(t *testing.T) {
	store := &murli.ProfileStore{Profiles: make(map[string]murli.Profile)}
	store.Set("staging", murli.Profile{Flags: map[string]string{"region": "eu-west-1"}})
	p, ok := store.Get("staging")
	if !ok {
		t.Fatal("staging not found")
	}
	if p.Flags["region"] != "eu-west-1" {
		t.Errorf("expected eu-west-1, got %q", p.Flags["region"])
	}
	_, ok2 := store.Get("nonexistent")
	if ok2 {
		t.Error("expected nonexistent to not be found")
	}
}

func TestProfileStoreDelete(t *testing.T) {
	store := &murli.ProfileStore{
		Default: "prod",
		Profiles: map[string]murli.Profile{
			"prod":    {Flags: map[string]string{"region": "us-east-1"}},
			"staging": {Flags: map[string]string{"region": "eu-west-1"}},
		},
	}
	// Delete non-existent is a no-op.
	store.Delete("ghost")

	// Delete the default — should clear Default.
	store.Delete("prod")
	if _, ok := store.Get("prod"); ok {
		t.Error("prod should be gone")
	}
	if store.Default != "" {
		t.Errorf("Default should be cleared, got %q", store.Default)
	}
	// staging still intact.
	if _, ok := store.Get("staging"); !ok {
		t.Error("staging should still exist")
	}
}

func TestProfileStoreSetDefault(t *testing.T) {
	store := &murli.ProfileStore{
		Profiles: map[string]murli.Profile{
			"prod": {Flags: map[string]string{"region": "us-east-1"}},
		},
	}
	if err := store.SetDefault("prod"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if store.Default != "prod" {
		t.Errorf("expected default=prod, got %q", store.Default)
	}
	// SetDefault on non-existent profile returns error.
	if err := store.SetDefault("ghost"); err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestProfileStoreSetDefaultIdempotent(t *testing.T) {
	store := &murli.ProfileStore{
		Default: "prod",
		Profiles: map[string]murli.Profile{
			"prod": {Flags: map[string]string{}},
		},
	}
	if err := store.SetDefault("prod"); err != nil {
		t.Fatalf("SetDefault twice: %v", err)
	}
	if store.Default != "prod" {
		t.Errorf("expected prod, got %q", store.Default)
	}
}

func TestProfileStoreNames(t *testing.T) {
	store := &murli.ProfileStore{
		Profiles: map[string]murli.Profile{
			"zzz":     {},
			"aaa":     {},
			"mmm":     {},
		},
	}
	names := store.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "aaa" || names[1] != "mmm" || names[2] != "zzz" {
		t.Errorf("expected sorted names, got %v", names)
	}
}

func TestProfilePathFormat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path := murli.ProfilePath("mytool")
	expected := filepath.Join(tmp, ".mytool", "profiles.json")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestProfileStoreSaveCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	store := &murli.ProfileStore{Profiles: map[string]murli.Profile{
		"prod": {Flags: map[string]string{"region": "us-east-1"}},
	}}
	// Directory doesn't exist yet.
	if err := store.Save("brandnewtool"); err != nil {
		t.Fatalf("Save should create directory: %v", err)
	}
	// Verify file exists.
	loaded, err := murli.LoadProfileStore("brandnewtool")
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if _, ok := loaded.Get("prod"); !ok {
		t.Error("prod profile not found after Save+Load")
	}
}
