package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatal(err)
		}
	})
}

func TestInitCreatesFileInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cmd := newInitCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "command.yaml")); err != nil {
		t.Fatalf("command.yaml not created: %v", err)
	}
}

func TestInitCreatesSubdirectory(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "deploy", "command.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("deploy/command.yaml not created: %v", err)
	}
}

func TestInitErrorsOnExistingFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile("command.yaml", []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when file exists")
	}
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile("command.yaml", []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	data, err := os.ReadFile("command.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "existing" {
		t.Fatal("file was not overwritten")
	}
}

func TestInitSubdirSlugNormalization(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"My Deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "my-deploy", "command.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("my-deploy/command.yaml not created: %v", err)
	}
}
