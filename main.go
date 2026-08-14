package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Justin-Stockton/deployqb/internal/deployqb"
	"github.com/Justin-Stockton/deployqb/internal/quickbase"
)

const help = `deployqb commands
init         Create qbcli.json from environment configuration
feat         Deploy to the feature environment
dev          Deploy to the development environment
prod         Deploy to the production environment
lfeat        Open the feature environment
ldev         Open the development environment
lprod        Open the production environment
efeatprefix  Edit the per-user feature prefix
genlinks     Display dbpage links for configured files
help         Display this help`

func main() {
	if err := deployqb.LoadDotEnv(".env"); err != nil {
		fatal(err)
	}
	command := "help"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if err := run(command); err != nil {
		fatal(err)
	}
}

func run(command string) error {
	switch command {
	case "init":
		return initProject()
	case "dev", "prod", "feat":
		return deploy(command)
	case "ldev", "lprod", "lfeat":
		return launch(strings.TrimPrefix(command, "l"))
	case "efeatprefix":
		return editFeaturePrefix()
	case "genlinks":
		return generateLinks()
	case "help", "":
		fmt.Println(help)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", command, help)
	}
}

func initProject() error {
	project, err := deployqb.ProjectFromEnv()
	if err != nil {
		return err
	}
	if existing, err := deployqb.LoadProject(deployqb.ProjectFilename); err == nil {
		project.URLQueryString = existing.URLQueryString
		project.Files = existing.Files
		project.LaunchProdPageID = existing.LaunchProdPageID
		project.LaunchDevPageID = existing.LaunchDevPageID
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := deployqb.SaveProject(deployqb.ProjectFilename, project); err != nil {
		return err
	}
	personal, filename, err := loadPersonal()
	if err != nil {
		return err
	}
	entry := personal[project.RepositoryID]
	if entry.FeaturePrefix == "" {
		entry.FeaturePrefix = envOr("CUSTOM_PREFIX_FEATURE", "F")
	}
	personal[project.RepositoryID] = entry
	if err := deployqb.SavePersonal(filename, personal); err != nil {
		return err
	}
	fmt.Println("Created qbcli.json. Update filesConf with the files to deploy.")
	return nil
}

func deploy(environment string) error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	if len(project.Files) == 0 {
		return errors.New("qbcli.json must list at least one file in filesConf")
	}
	personal, personalFilename, err := loadPersonal()
	if err != nil {
		return err
	}
	personalProject, ok := personal[project.RepositoryID]
	if !ok {
		return errors.New("project is not initialized; run deployqb init")
	}
	if environment == "dev" || environment == "prod" {
		fmt.Printf("Deploy to %s? Type yes to continue: ", environment)
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
			fmt.Println("Deployment cancelled.")
			return nil
		}
	}
	prefix, err := deployqb.Prefix(project, personalProject, environment)
	if err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	prepared, err := deployqb.PrepareFiles(root, project, prefix)
	if err != nil {
		return err
	}

	separateDevApp := project.DevAndProdQuickBaseApplications == "yes" && environment != "prod"
	dbid := project.DBID
	userToken := envOr("QB_USERTOKEN", os.Getenv("USERTOKEN"))
	appToken := envOr("QB_APP_TOKEN", os.Getenv("APP_TOKEN"))
	if separateDevApp {
		dbid = project.DevDBID
		userToken = envOr("QB_DEV_USERTOKEN", envOr("DEV_USERTOKEN", userToken))
		appToken = envOr("QB_DEV_APP_TOKEN", envOr("DEV_APP_TOKEN", appToken))
	}
	if dbid == "" || userToken == "" {
		return errors.New("selected environment is missing a DBID or user token")
	}

	client := quickbase.Client{}
	for _, file := range prepared {
		pageID, err := client.AddReplaceDBPage(dbid, project.Realm, userToken, appToken, file.Name, file.Body)
		if err != nil {
			return fmt.Errorf("deploy %s: %w", file.Name, err)
		}
		if file.IsIndex {
			switch environment {
			case "prod":
				project.LaunchProdPageID = pageID
			case "dev":
				project.LaunchDevPageID = pageID
			case "feat":
				personalProject.LaunchFeatPageID = pageID
				personal[project.RepositoryID] = personalProject
			}
		}
	}
	if err := deployqb.SaveProject(deployqb.ProjectFilename, project); err != nil {
		return err
	}
	if err := deployqb.SavePersonal(personalFilename, personal); err != nil {
		return err
	}
	fmt.Printf("Deployed %d files to %s.\n", len(prepared), environment)
	return nil
}

func launch(environment string) error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	personal, _, err := loadPersonal()
	if err != nil {
		return err
	}
	dbid, pageID := project.DBID, project.LaunchProdPageID
	if environment != "prod" {
		if project.DevAndProdQuickBaseApplications == "yes" {
			dbid = project.DevDBID
		}
		if environment == "dev" {
			pageID = project.LaunchDevPageID
		} else {
			pageID = personal[project.RepositoryID].LaunchFeatPageID
		}
	}
	if pageID == 0 {
		return fmt.Errorf("deploy %s with an isIndexFile entry before launching it", environment)
	}
	address, err := launchAddress(project.Realm, dbid, pageID, project.URLQueryString)
	if err != nil {
		return err
	}
	return openBrowser(address)
}

func launchAddress(realm, dbid string, pageID int, rawQuery string) (string, error) {
	query, err := url.ParseQuery(strings.TrimLeft(rawQuery, "?&"))
	if err != nil {
		return "", fmt.Errorf("invalid urlQueryString: %w", err)
	}
	query.Set("a", "dbpage")
	query.Set("pageID", fmt.Sprint(pageID))
	return fmt.Sprintf("%s/db/%s?%s", quickbase.NormalizeRealm(realm), url.PathEscape(dbid), query.Encode()), nil
}

func editFeaturePrefix() error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	personal, filename, err := loadPersonal()
	if err != nil {
		return err
	}
	entry, ok := personal[project.RepositoryID]
	if !ok {
		return errors.New("project is not initialized; run deployqb init")
	}
	fmt.Print("Enter a new feature prefix: ")
	value, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("prefix cannot be empty")
	}
	entry.FeaturePrefix = value
	personal[project.RepositoryID] = entry
	return deployqb.SavePersonal(filename, personal)
}

func generateLinks() error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	personal, _, err := loadPersonal()
	if err != nil {
		return err
	}
	for _, environment := range []string{"dev", "prod", "feat"} {
		prefix, err := deployqb.Prefix(project, personal[project.RepositoryID], environment)
		if err != nil {
			return err
		}
		fmt.Printf("\n%s:\n", environment)
		for _, file := range project.Files {
			fmt.Printf("?a=dbpage&pagename=%s%s\n", prefix, file.Filename)
		}
	}
	return nil
}

func loadProject() (deployqb.Project, error) {
	project, err := deployqb.LoadProject(deployqb.ProjectFilename)
	if errors.Is(err, os.ErrNotExist) {
		return project, errors.New("run deployqb from a project root containing qbcli.json")
	}
	return project, err
}

func loadPersonal() (deployqb.PersonalConfig, string, error) {
	filename, err := deployqb.PersonalConfigPath()
	if err != nil {
		return nil, "", err
	}
	config, err := deployqb.LoadPersonal(filename)
	return config, filename, err
}

func openBrowser(address string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", address)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	return command.Start()
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "deployqb:", err)
	os.Exit(1)
}
