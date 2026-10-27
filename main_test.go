package main

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := deployqb.SaveProject(deployqb.ProjectFilename, deployqb.Project{
		RepositoryID: "repo", DBID: "db", Realm: "acme", ProdPrefix: "P", DevPrefix: "D",
		Files: []deployqb.FileConfig{{Filename: "index.html", IsIndexFile: true}}, LaunchProdPageID: 11, LaunchDevPageID: 12,
	}); err != nil {
		t.Fatal(err)
	}
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
	if project.RepositoryID != "repo" || project.DBID != "db" || len(project.Files) != 1 || project.LaunchProdPageID != 11 || project.LaunchDevPageID != 12 {
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

func TestInitProjectInteractiveStoresProfiles(t *testing.T) {
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

	input := strings.NewReader(strings.Join([]string{
		"repo", "prod-db", "yes", "dev-db", "P", "D",
		"acme-prod", "acme", "prod-token", "prod-app",
		"acme-dev", "acme", "dev-token", "",
	}, "\n") + "\n")
	var output strings.Builder
	if err := initProjectWithIO(input, &output); err != nil {
		t.Fatal(err)
	}
	project, err := deployqb.LoadProject(deployqb.ProjectFilename)
	if err != nil {
		t.Fatal(err)
	}
	if project.ProdProfile != "acme-prod" || project.DevProfile != "acme-dev" {
		t.Fatalf("profile references = %q/%q", project.ProdProfile, project.DevProfile)
	}
	filename, err := deployqb.PersonalConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	config, err := deployqb.LoadUserConfig(filename)
	if err != nil {
		t.Fatal(err)
	}
	if config.Profiles["acme-prod"].UserToken != "prod-token" || config.Profiles["acme-dev"].UserToken != "dev-token" {
		t.Fatalf("profiles = %#v", config.Profiles)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
	if strings.Contains(output.String(), "prod-token") || strings.Contains(output.String(), "prod-app") {
		t.Fatal("interactive output leaked a credential")
	}
}

func TestInitProjectRejectsInvalidSeparateApplicationAnswer(t *testing.T) {
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

	err = initProjectWithIO(strings.NewReader("repo\nprod-db\nyess\n"), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "must be yes or no") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(deployqb.ProjectFilename); !os.IsNotExist(err) {
		t.Fatalf("project file created after invalid input: %v", err)
	}
}

func TestCredentialsSelectEnvironmentProfile(t *testing.T) {
	t.Setenv("QB_APP_TOKEN", "environment-app-token")
	project := deployqb.Project{ProdProfile: "prod", DevProfile: "dev", Realm: "legacy"}
	config := deployqb.UserConfig{Profiles: map[string]deployqb.Profile{
		"prod": {Realm: "prod-realm", UserToken: "prod-token"},
		"dev":  {Realm: "dev-realm", UserToken: "dev-token"},
	}}
	if realm, token, appToken, err := credentials(project, config, false); err != nil || realm != "prod-realm" || token != "prod-token" || appToken != "" {
		t.Fatalf("production credentials = %q/%q/%q/%v", realm, token, appToken, err)
	}
	if realm, token, _, err := credentials(project, config, true); err != nil || realm != "dev-realm" || token != "dev-token" {
		t.Fatalf("development credentials = %q/%q/%v", realm, token, err)
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
