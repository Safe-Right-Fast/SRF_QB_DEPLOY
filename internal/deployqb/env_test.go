package deployqb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvDoesNotOverwriteEnvironment(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(filename, []byte("DEPLOYQB_TEST_ONE=from-file\nDEPLOYQB_TEST_TWO='quoted value'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEPLOYQB_TEST_ONE", "from-environment")
	_ = os.Unsetenv("DEPLOYQB_TEST_TWO")
	t.Cleanup(func() { _ = os.Unsetenv("DEPLOYQB_TEST_TWO") })
	if err := LoadDotEnv(filename); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DEPLOYQB_TEST_ONE"); got != "from-environment" {
		t.Fatalf("existing value overwritten: %q", got)
	}
	if got := os.Getenv("DEPLOYQB_TEST_TWO"); got != "quoted value" {
		t.Fatalf("quoted value = %q", got)
	}
}
