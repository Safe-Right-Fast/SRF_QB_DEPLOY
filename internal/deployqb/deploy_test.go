package deployqb

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestPrepareFilesResolvesBundlerAssets(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "dist", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	js := []byte("console.log('built')")
	if err := os.WriteFile(filepath.Join(assets, "index-abc123.js"), js, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index-def456.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	html := `<link rel="stylesheet" href="/assets/index-def456.css"><script src="/assets/index-abc123.js"></script>`
	if err := os.WriteFile(filepath.Join(root, "dist", "index.html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	project := Project{Files: []FileConfig{
		{Filename: "app.js", Path: "dist/assets", Source: "*.js"},
		{Filename: "app.css", Path: "dist/assets", Source: "*.css"},
		{Filename: "index.html", Path: "dist", Dependencies: []int{0, 1}, IsIndexFile: true},
	}}

	files, err := PrepareFiles(root, project, "P_repo_")
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Name != "P_repo_app.js" || string(files[0].Body) != string(js) {
		t.Fatalf("unexpected JS file: %#v", files[0])
	}
	jsHash := sha256.Sum256(js)
	wantJS := fmt.Sprintf(`?a=dbpage&pagename=P_repo_app.js&amp;v=%x`, jsHash[:6])
	if body := string(files[2].Body); !strings.Contains(body, wantJS) || strings.Contains(body, "/assets/") {
		t.Fatalf("rewritten HTML = %q", body)
	}
}

func TestPrepareFilesRejectsAmbiguousSourceGlob(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.js"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "two.js"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := Project{Files: []FileConfig{{Filename: "app.js", Source: "*.js"}}}
	if _, err := PrepareFiles(root, project, "P_repo_"); err == nil || !strings.Contains(err.Error(), "matched 2 files") {
		t.Fatalf("error = %v", err)
	}
}

func TestPrepareFilesRejectsPathsOutsideProject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := Project{Files: []FileConfig{{Filename: "secret", Path: ".."}}}
	if _, err := PrepareFiles(root, project, "D_repo_"); err == nil {
		t.Fatal("path traversal should be rejected")
	}
}

func TestPrepareFilesRejectsSymlinksOutsideProject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "secret")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	project := Project{Files: []FileConfig{{Filename: "linked"}}}
	if _, err := PrepareFiles(root, project, "D_repo_"); err == nil {
		t.Fatal("escaping symlink should be rejected")
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
