package deployqb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ProjectFilename = "qbcli.json"

type FlexibleBool bool

func (b *FlexibleBool) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch value := value.(type) {
	case bool:
		*b = FlexibleBool(value)
	case string:
		*b = FlexibleBool(value == "yes" || value == "true")
	default:
		return fmt.Errorf("expected boolean or string")
	}
	return nil
}

func (b FlexibleBool) MarshalJSON() ([]byte, error) { return json.Marshal(bool(b)) }

type FileConfig struct {
	Filename     string       `json:"filename"`
	Path         string       `json:"path,omitempty"`
	Dependencies []int        `json:"dependencies,omitempty"`
	IsIndexFile  FlexibleBool `json:"isIndexFile,omitempty"`
}

type Project struct {
	URLQueryString                  string       `json:"urlQueryString,omitempty"`
	RepositoryID                    string       `json:"repositoryId"`
	ProdPrefix                      string       `json:"prodPrefix"`
	DevPrefix                       string       `json:"devPrefix"`
	DBID                            string       `json:"dbid"`
	DevDBID                         string       `json:"devDbid,omitempty"`
	DevAndProdQuickBaseApplications string       `json:"devAndProdQuickBaseApplications"`
	Realm                           string       `json:"realm"`
	Files                           []FileConfig `json:"filesConf"`
	LaunchProdPageID                int          `json:"launchProdPageId,omitempty"`
	LaunchDevPageID                 int          `json:"launchDevPageId,omitempty"`
}

type PersonalProject struct {
	FeaturePrefix    string `json:"featurePrefix"`
	LaunchFeatPageID int    `json:"launchFeatPageId,omitempty"`
}

type PersonalConfig map[string]PersonalProject

func LoadProject(filename string) (Project, error) {
	var project Project
	data, err := os.ReadFile(filename)
	if err != nil {
		return project, err
	}
	if err := json.Unmarshal(data, &project); err != nil {
		return project, fmt.Errorf("read %s: %w", filename, err)
	}
	return project, nil
}

func SaveProject(filename string, project Project) error { return saveJSON(filename, project) }

func PersonalConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "deployqb", "config.json"), nil
}

func LoadPersonal(filename string) (PersonalConfig, error) {
	var config PersonalConfig
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return PersonalConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func SavePersonal(filename string, config PersonalConfig) error { return saveJSON(filename, config) }

func saveJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filename, append(data, '\n'), 0o600)
}
