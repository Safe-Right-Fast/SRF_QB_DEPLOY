package deployqb

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func LoadDotEnv(filename string) error {
	file, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid .env line %q", line)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
			quote := value[0]
			if end := strings.IndexByte(value[1:], quote); end >= 0 {
				value = value[1 : end+1]
			}
		} else if comment := strings.IndexByte(value, '#'); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func ProjectFromEnv() (Project, error) {
	project := Project{
		RepositoryID:                    os.Getenv("REPOSITORY_ID"),
		ProdPrefix:                      envOr("CUSTOM_PREFIX_PRODUCTION", "P"),
		DevPrefix:                       envOr("CUSTOM_PREFIX", "D"),
		DBID:                            envOr("QB_DBID", os.Getenv("DBID")),
		DevDBID:                         envOr("QB_DEV_DBID", os.Getenv("DEV_DBID")),
		DevAndProdQuickBaseApplications: envOr("DEV_AND_PROD_QUICKBASE_APPLICATIONS", "no"),
		Realm:                           envOr("QB_REALM", os.Getenv("REALM")),
		Files:                           []FileConfig{},
	}
	var missing []string
	for name, value := range map[string]string{"REPOSITORY_ID": project.RepositoryID, "QB_DBID": project.DBID, "QB_REALM": project.Realm} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return project, fmt.Errorf("missing required environment configuration: %s", strings.Join(missing, ", "))
	}
	return project, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
