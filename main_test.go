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
	personalPath, _ := deployqb.PersonalConfigPath()
	if err := deployqb.SavePersonal(personalPath, deployqb.PersonalConfig{
		"repo": {FeaturePrefix: "OLD", LaunchFeatPageID: 7},
	}); err != nil {
		t.Fatal(err)
	}

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
	personal, err := deployqb.LoadPersonal(personalPath)
	if err != nil {
		t.Fatal(err)
	}
	if personal["repo"].FeaturePrefix != "OLD" || personal["repo"].LaunchFeatPageID != 7 {
		t.Fatalf("existing personal config was not preserved: %#v", personal)
	}
}

func TestLaunchAddressEncodesQueryValues(t *testing.T) {
	address, err := launchAddress("acme", "db id", 42, "name=hello world&next=%2Fhome")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://acme.quickbase.com/db/db%20id?a=dbpage&name=hello+world&next=%2Fhome&pageID=42"
	if address != want {
		t.Fatalf("address = %q, want %q", address, want)
	}
}
