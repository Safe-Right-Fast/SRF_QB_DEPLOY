package deployqb

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DeploymentFile struct {
	Name    string
	Body    []byte
	IsIndex bool
}

type resolvedFile struct {
	config FileConfig
	path   string
	body   []byte
}

func Prefix(project Project, personal PersonalProject, environment string) (string, error) {
	prefix, fallback := "", ""
	switch environment {
	case "dev":
		prefix, fallback = project.DevPrefix, "D"
	case "prod":
		prefix, fallback = project.ProdPrefix, "P"
	case "feat":
		prefix, fallback = personal.FeaturePrefix, "F"
	default:
		return "", fmt.Errorf("unknown environment %q", environment)
	}
	if prefix == "" {
		prefix = fallback
	}
	return fmt.Sprintf("%s_%s_", prefix, project.RepositoryID), nil
}

func PrepareFiles(root string, project Project, prefix string) ([]DeploymentFile, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolved := make([]resolvedFile, len(project.Files))
	for index, file := range project.Files {
		path, err := resolveSource(root, file)
		if err != nil {
			return nil, err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Filename, err)
		}
		body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
		if len(body) == 0 {
			return nil, fmt.Errorf("%s is empty", file.Filename)
		}
		resolved[index] = resolvedFile{config: file, path: path, body: body}
	}

	prepared := make([]DeploymentFile, 0, len(resolved))
	for _, file := range resolved {
		body := file.body
		for _, dependencyIndex := range file.config.Dependencies {
			if dependencyIndex < 0 || dependencyIndex >= len(resolved) {
				return nil, fmt.Errorf("%s has invalid dependency index %d", file.config.Filename, dependencyIndex)
			}
			dependency := resolved[dependencyIndex]
			pageName := prefix + dependency.config.Filename
			body = bytes.ReplaceAll(body, []byte("pagename="+dependency.config.Filename), []byte("pagename="+pageName))
			if dependency.config.Source != "" {
				body = rewriteBuiltAsset(body, file.path, dependency, pageName)
			}
		}
		prepared = append(prepared, DeploymentFile{Name: prefix + file.config.Filename, Body: body, IsIndex: bool(file.config.IsIndexFile)})
	}
	return prepared, nil
}

func resolveSource(root string, file FileConfig) (string, error) {
	source := file.Source
	if source == "" {
		source = file.Filename
	}
	pattern := filepath.Join(root, file.Path, source)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("source %s: %w", source, err)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("%s source %q matched %d files, want 1", file.Filename, source, len(matches))
	}
	path, err := filepath.EvalSymlinks(matches[0])
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file.Filename, err)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s resolves outside the project root", file.Filename)
	}
	return path, nil
}

func rewriteBuiltAsset(body []byte, parentPath string, dependency resolvedFile, pageName string) []byte {
	reference, err := filepath.Rel(filepath.Dir(parentPath), dependency.path)
	if err != nil {
		return body
	}
	reference = filepath.ToSlash(reference)
	sum := sha256.Sum256(dependency.body)
	pageURL := fmt.Sprintf("?a=dbpage&pagename=%s&amp;v=%x", pageName, sum[:6])
	for _, candidate := range []string{"/" + reference, "./" + reference, reference} {
		for _, quote := range []string{"\"", "'"} {
			old := []byte(quote + candidate + quote)
			body = bytes.ReplaceAll(body, old, []byte(quote+pageURL+quote))
		}
	}
	return body
}
