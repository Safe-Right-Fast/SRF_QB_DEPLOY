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

type FileConfig struct {
	Filename     string       `json:"filename"`
	Path         string       `json:"path,omitempty"`
	Source       string       `json:"source,omitempty"`
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
	Realm                           string       `json:"realm,omitempty"`
	ProdProfile                     string       `json:"prodProfile,omitempty"`
	DevProfile                      string       `json:"devProfile,omitempty"`
	Files                           []FileConfig `json:"filesConf"`
	LaunchProdPageID                int          `json:"launchProdPageId,omitempty"`
	LaunchDevPageID                 int          `json:"launchDevPageId,omitempty"`
}

type PersonalProject struct {
	FeaturePrefix    string `json:"customPrefixFeature"`
	LaunchFeatPageID int    `json:"launchFeatPageId,omitempty"`
}

type PersonalConfig map[string]PersonalProject

type Profile struct {
	Realm     string `json:"realm"`
	UserToken string `json:"userToken"`
	AppToken  string `json:"appToken,omitempty"`
}

type UserConfig struct {
	Profiles map[string]Profile `json:"profiles,omitempty"`
	Personal PersonalConfig     `json:"personal,omitempty"`
}

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

func LoadUserConfig(filename string) (UserConfig, error) {
	var config UserConfig
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return UserConfig{Profiles: map[string]Profile{}, Personal: PersonalConfig{}}, nil
	}
	if err != nil {
		return config, err
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(data, &sections); err != nil {
		return config, fmt.Errorf("read %s: %w", filename, err)
	}
	if _, hasProfiles := sections["profiles"]; hasProfiles {
		if err := json.Unmarshal(data, &config); err != nil {
			return config, fmt.Errorf("read %s: %w", filename, err)
		}
	} else if _, hasPersonal := sections["personal"]; hasPersonal {
		if err := json.Unmarshal(data, &config); err != nil {
			return config, fmt.Errorf("read %s: %w", filename, err)
		}
	} else {
		if err := json.Unmarshal(data, &config.Personal); err != nil {
			return config, fmt.Errorf("read %s: %w", filename, err)
		}
	}
	if config.Profiles == nil {
		config.Profiles = map[string]Profile{}
	}
	if config.Personal == nil {
		config.Personal = PersonalConfig{}
	}
	return config, nil
}

func SaveUserConfig(filename string, config UserConfig) error {
	if config.Profiles == nil {
		config.Profiles = map[string]Profile{}
	}
	if config.Personal == nil {
		config.Personal = PersonalConfig{}
	}
	return saveJSON(filename, config)
}

func PersonalConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "configstore", "deployqb.json"), nil
}

func LoadPersonal(filename string) (PersonalConfig, error) {
	config, err := LoadUserConfig(filename)
	return config.Personal, err
}

func SavePersonal(filename string, personal PersonalConfig) error {
	config, err := LoadUserConfig(filename)
	if err != nil {
		return err
	}
	config.Personal = personal
	return SaveUserConfig(filename, config)
}

func saveJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(filename); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to replace symlink %s", filename)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".deployqb-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err = temporary.Write(append(data, '\n')); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}
