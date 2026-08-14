package deployqb

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

type DeploymentFile struct {
	Name    string
	Body    []byte
	IsIndex bool
}

func Prefix(project Project, personal PersonalProject, environment string) (string, error) {
	prefix := map[string]string{
		"dev":  project.DevPrefix,
		"prod": project.ProdPrefix,
		"feat": personal.FeaturePrefix,
	}[environment]
	defaults := map[string]string{"dev": "D", "prod": "P", "feat": "F"}
	fallback, ok := defaults[environment]
	if !ok {
		return "", fmt.Errorf("unknown environment %q", environment)
	}
	if prefix == "" {
		prefix = fallback
	}
	return fmt.Sprintf("%s_%s_", prefix, project.RepositoryID), nil
}

func PrepareFiles(root string, project Project, prefix string) ([]DeploymentFile, error) {
	prepared := make([]DeploymentFile, 0, len(project.Files))
	for _, file := range project.Files {
		body, err := os.ReadFile(filepath.Join(root, file.Path, file.Filename))
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
