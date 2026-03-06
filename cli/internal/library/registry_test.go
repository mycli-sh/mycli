package library

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLoadRegistryEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg.Sources) != 0 {
		t.Fatalf("expected empty registry, got %d sources", len(reg.Sources))
	}
}

func TestLoadRegistryRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	now := time.Now().Truncate(time.Second)
	original := &SourceRegistry{
		Sources: []SourceEntry{
			{
				Name:        "test-lib",
				Owner:       "testuser",
				Slug:        "test-lib",
				Kind:        "git",
				GitURL:      "https://example.com/repo.git",
				AddedAt:     now,
				LastUpdated: now,
			},
			{
				Name:        "other-lib",
				Slug:        "other-lib",
				Kind:        "registry",
				AddedAt:     now,
				LastUpdated: now,
			},
		},
	}

	if err := SaveRegistry(original); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	loaded, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(loaded.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(loaded.Sources))
	}
	if loaded.Sources[0].Name != "test-lib" {
		t.Errorf("expected name 'test-lib', got %q", loaded.Sources[0].Name)
	}
	if loaded.Sources[0].GitURL != "https://example.com/repo.git" {
		t.Errorf("expected git_url, got %q", loaded.Sources[0].GitURL)
	}
	if loaded.Sources[1].Kind != "registry" {
		t.Errorf("expected kind 'registry', got %q", loaded.Sources[1].Kind)
	}
}

func TestSaveRegistryCreatesDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	reg := &SourceRegistry{
		Sources: []SourceEntry{{Name: "a", Slug: "a", Kind: "git"}},
	}
	if err := SaveRegistry(reg); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	// Verify the file exists and is valid JSON
	data, err := os.ReadFile(RegistryPath())
	if err != nil {
		t.Fatalf("read registry file: %v", err)
	}
	var check SourceRegistry
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("invalid JSON in registry file: %v", err)
	}
	if len(check.Sources) != 1 {
		t.Fatalf("expected 1 source in file, got %d", len(check.Sources))
	}
}

func TestFindByName(t *testing.T) {
	reg := &SourceRegistry{
		Sources: []SourceEntry{
			{Name: "alpha", Slug: "alpha"},
			{Name: "beta", Slug: "beta"},
		},
	}

	found := FindByName(reg, "alpha")
	if found == nil {
		t.Fatal("expected to find 'alpha'")
	}
	if found.Slug != "alpha" {
		t.Errorf("expected slug 'alpha', got %q", found.Slug)
	}

	notFound := FindByName(reg, "gamma")
	if notFound != nil {
		t.Error("expected nil for non-existent name")
	}
}

func TestFindByOwnerSlug(t *testing.T) {
	reg := &SourceRegistry{
		Sources: []SourceEntry{
			{Name: "a", Owner: "user1", Slug: "lib-a"},
			{Name: "b", Owner: "user2", Slug: "lib-b"},
		},
	}

	found := FindByOwnerSlug(reg, "user1", "lib-a")
	if found == nil {
		t.Fatal("expected to find user1/lib-a")
	}
	if found.Name != "a" {
		t.Errorf("expected name 'a', got %q", found.Name)
	}

	// Wrong owner
	notFound := FindByOwnerSlug(reg, "user1", "lib-b")
	if notFound != nil {
		t.Error("expected nil for wrong owner/slug combination")
	}

	// Non-existent
	notFound = FindByOwnerSlug(reg, "nobody", "nothing")
	if notFound != nil {
		t.Error("expected nil for non-existent entry")
	}
}

func TestFindBySlug(t *testing.T) {
	reg := &SourceRegistry{
		Sources: []SourceEntry{
			{Name: "a", Slug: "lib-a"},
			{Name: "b", Slug: "lib-b"},
		},
	}

	found := FindBySlug(reg, "lib-b")
	if found == nil {
		t.Fatal("expected to find 'lib-b'")
	}
	if found.Name != "b" {
		t.Errorf("expected name 'b', got %q", found.Name)
	}

	notFound := FindBySlug(reg, "lib-c")
	if notFound != nil {
		t.Error("expected nil for non-existent slug")
	}
}

func TestRemove(t *testing.T) {
	reg := &SourceRegistry{
		Sources: []SourceEntry{
			{Name: "alpha", Slug: "alpha"},
			{Name: "beta", Slug: "beta"},
			{Name: "gamma", Slug: "gamma"},
		},
	}

	removed := Remove(reg, "beta")
	if !removed {
		t.Error("expected Remove to return true")
	}
	if len(reg.Sources) != 2 {
		t.Fatalf("expected 2 sources after remove, got %d", len(reg.Sources))
	}
	if reg.Sources[0].Name != "alpha" || reg.Sources[1].Name != "gamma" {
		t.Errorf("unexpected sources after remove: %v", reg.Sources)
	}

	notRemoved := Remove(reg, "nonexistent")
	if notRemoved {
		t.Error("expected Remove to return false for non-existent entry")
	}
	if len(reg.Sources) != 2 {
		t.Fatalf("expected 2 sources unchanged, got %d", len(reg.Sources))
	}
}
