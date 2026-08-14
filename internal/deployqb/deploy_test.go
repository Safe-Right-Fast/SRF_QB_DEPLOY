package deployqb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "site.css"), []byte("body {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build", "index.html"), []byte("\xEF\xBB\xBF?pagename=site.css"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := Project{Files: []FileConfig{
		{Filename: "site.css"},
		{Filename: "index.html", Path: "build", Dependencies: []int{0}, IsIndexFile: true},
	}}

	files, err := PrepareFiles(root, project, "D_repo_")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(files[1].Body), "?pagename=D_repo_site.css"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if files[1].Name != "D_repo_index.html" || !files[1].IsIndex {
		t.Fatalf("unexpected index file: %#v", files[1])
	}
}

func TestProjectAcceptsLegacyIndexString(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "qbcli.json")
	data := `{"repositoryId":"repo","filesConf":[{"filename":"index.html","isIndexFile":"yes"}]}`
	if err := os.WriteFile(filename, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	project, err := LoadProject(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bool(project.Files[0].IsIndexFile) {
		t.Fatal("legacy yes should be true")
	}
}

func TestPrefix(t *testing.T) {
	project := Project{RepositoryID: "repo", DevPrefix: "DEV"}
	if got, _ := Prefix(project, PersonalProject{}, "dev"); got != "DEV_repo_" {
		t.Fatalf("prefix = %q", got)
	}
	if got, _ := Prefix(project, PersonalProject{}, "feat"); got != "F_repo_" {
		t.Fatalf("prefix = %q", got)
	}
}
