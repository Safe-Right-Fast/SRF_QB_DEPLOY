package deployqb

import (
	"bytes"
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
	prepared := make([]DeploymentFile, 0, len(project.Files))
	for _, file := range project.Files {
		filename, err := filepath.EvalSymlinks(filepath.Join(root, file.Path, file.Filename))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Filename, err)
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("%s resolves outside the project root", file.Filename)
		}
		body, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Filename, err)
		}
		body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
		if len(body) == 0 {
			return nil, fmt.Errorf("%s is empty", file.Filename)
		}
		for _, dependencyIndex := range file.Dependencies {
			if dependencyIndex < 0 || dependencyIndex >= len(project.Files) {
				return nil, fmt.Errorf("%s has invalid dependency index %d", file.Filename, dependencyIndex)
			}
			dependency := project.Files[dependencyIndex].Filename
			body = bytes.ReplaceAll(body, []byte("pagename="+dependency), []byte("pagename="+prefix+dependency))
		}
		prepared = append(prepared, DeploymentFile{Name: prefix + file.Filename, Body: body, IsIndex: bool(file.IsIndexFile)})
	}
	return prepared, nil
}
