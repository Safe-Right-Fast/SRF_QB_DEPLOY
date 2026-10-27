package deployqb

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserConfigLoadsLegacyPersonalConfig(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "deployqb.json")
	legacy := `{"repo":{"customPrefixFeature":"OLD","launchFeatPageId":7}}`
	if err := os.WriteFile(filename, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	oldFile, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer oldFile.Close()
	config, err := LoadUserConfig(filename)
	if err != nil {
		t.Fatal(err)
	}
	if config.Personal["repo"].FeaturePrefix != "OLD" || config.Personal["repo"].LaunchFeatPageID != 7 {
		t.Fatalf("personal config = %#v", config.Personal)
	}
	config.Profiles["acme"] = Profile{Realm: "acme", UserToken: "token"}
	if err := SaveUserConfig(filename, config); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadUserConfig(filename)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Profiles["acme"].UserToken != "token" || reloaded.Personal["repo"].FeaturePrefix != "OLD" {
		t.Fatalf("reloaded config = %#v", reloaded)
	}
	oldData, err := io.ReadAll(oldFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oldData), "token") {
		t.Fatal("open descriptor observed credentials written during migration")
	}
	if info, err := os.Stat(filename); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o, want 600", info.Mode().Perm())
	}
}
