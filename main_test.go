package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Justin-Stockton/deployqb/internal/deployqb"
)

func TestInitProjectCreatesProjectAndPersonalConfiguration(t *testing.T) {
	root := t.TempDir()
	oldDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDirectory) })
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("REPOSITORY_ID", "repo")
	t.Setenv("QB_DBID", "db")
	t.Setenv("QB_REALM", "acme")
	t.Setenv("CUSTOM_PREFIX_FEATURE", "ME")

	if err := initProject(); err != nil {
		t.Fatal(err)
	}
	project, err := deployqb.LoadProject(deployqb.ProjectFilename)
	if err != nil {
		t.Fatal(err)
	}
	if project.RepositoryID != "repo" || project.DBID != "db" {
		t.Fatalf("unexpected project: %#v", project)
	}
	personalPath, _ := deployqb.PersonalConfigPath()
	personal, err := deployqb.LoadPersonal(personalPath)
	if err != nil {
		t.Fatal(err)
	}
	if personal["repo"].FeaturePrefix != "ME" {
		t.Fatalf("unexpected personal config: %#v", personal)
	}
}
