package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/Justin-Stockton/deployqb/internal/deployqb"
	"github.com/Justin-Stockton/deployqb/internal/quickbase"
	"golang.org/x/term"
)

const help = `deployqb commands
init         Create or refresh qbcli.json interactively
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

type initPrompter struct {
	reader   *bufio.Reader
	writer   io.Writer
	terminal *os.File
}

func newInitPrompter(reader io.Reader, writer io.Writer) *initPrompter {
	prompter := &initPrompter{reader: bufio.NewReader(reader), writer: writer}
	if file, ok := reader.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		prompter.terminal = file
	}
	return prompter
}

func (p *initPrompter) ask(label, fallback string) (string, error) {
	if fallback == "" {
		fmt.Fprintf(p.writer, "%s: ", label)
	} else {
		fmt.Fprintf(p.writer, "%s [%s]: ", label, fallback)
	}
	value, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return value, nil
}

func (p *initPrompter) askSecret(label, fallback string) (string, error) {
	if fallback == "" {
		fmt.Fprintf(p.writer, "%s: ", label)
	} else {
		fmt.Fprintf(p.writer, "%s [set]: ", label)
	}
	if p.terminal != nil {
		value, err := term.ReadPassword(int(p.terminal.Fd()))
		fmt.Fprintln(p.writer)
		if err != nil {
			return "", err
		}
		if len(value) == 0 {
			return fallback, nil
		}
		return string(value), nil
	}
	value, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return value, nil
}

func initProject() error { return initProjectWithIO(os.Stdin, os.Stdout) }

func initProjectWithIO(reader io.Reader, writer io.Writer) error {
	prompter := newInitPrompter(reader, writer)
	project, err := initialProject()
	if err != nil {
		return err
	}
	userConfig, userConfigFilename, err := loadUserConfig()
	if err != nil {
		return err
	}

	if project.RepositoryID, err = prompter.ask("Repository ID", firstNonEmpty(project.RepositoryID, os.Getenv("REPOSITORY_ID"))); err != nil {
		return err
	}
	if project.DBID, err = prompter.ask("Production DBID", firstNonEmpty(project.DBID, envOr("QB_DBID", os.Getenv("DBID")))); err != nil {
		return err
	}
	separateDefault := firstNonEmpty(project.DevAndProdQuickBaseApplications, envOr("DEV_AND_PROD_QUICKBASE_APPLICATIONS", "no"))
	if project.DevAndProdQuickBaseApplications, err = prompter.ask("Use separate development application (yes/no)", separateDefault); err != nil {
		return err
	}
	project.DevAndProdQuickBaseApplications = yesNo(project.DevAndProdQuickBaseApplications)
	if project.DevAndProdQuickBaseApplications != "yes" && project.DevAndProdQuickBaseApplications != "no" {
		return errors.New("separate development application must be yes or no")
	}
	if project.DevAndProdQuickBaseApplications == "yes" {
		if project.DevDBID, err = prompter.ask("Development DBID", firstNonEmpty(project.DevDBID, envOr("QB_DEV_DBID", os.Getenv("DEV_DBID")))); err != nil {
			return err
		}
	}
	if project.ProdPrefix, err = prompter.ask("Production page prefix", firstNonEmpty(project.ProdPrefix, envOr("CUSTOM_PREFIX_PRODUCTION", "P"))); err != nil {
		return err
	}
	if project.DevPrefix, err = prompter.ask("Development page prefix", firstNonEmpty(project.DevPrefix, envOr("CUSTOM_PREFIX", "D"))); err != nil {
		return err
	}
	if project.RepositoryID == "" || project.DBID == "" {
		return errors.New("repository ID and production DBID are required")
	}
	if project.DevAndProdQuickBaseApplications == "yes" && project.DevDBID == "" {
		return errors.New("development DBID is required when using a separate development application")
	}

	prodDefaults := deployqb.Profile{
		Realm:     firstNonEmpty(project.Realm, envOr("QB_REALM", os.Getenv("REALM"))),
		UserToken: envOr("QB_USERTOKEN", os.Getenv("USERTOKEN")),
		AppToken:  envOr("QB_APP_TOKEN", os.Getenv("APP_TOKEN")),
	}
	project.Realm = firstNonEmpty(project.Realm, prodDefaults.Realm)
	project.ProdProfile, err = configureProfile(prompter, userConfig.Profiles, "Production", firstNonEmpty(project.ProdProfile, defaultProfile(userConfig.Profiles, "production")), prodDefaults)
	if err != nil {
		return err
	}
	if project.DevAndProdQuickBaseApplications == "yes" {
		devDefaults := deployqb.Profile{
			Realm:     firstNonEmpty(envOr("QB_DEV_REALM", ""), project.Realm, prodDefaults.Realm),
			UserToken: firstNonEmpty(envOr("QB_DEV_USERTOKEN", envOr("DEV_USERTOKEN", "")), prodDefaults.UserToken),
			AppToken:  envOr("QB_DEV_APP_TOKEN", envOr("DEV_APP_TOKEN", "")),
		}
		project.DevProfile, err = configureProfile(prompter, userConfig.Profiles, "Development", firstNonEmpty(project.DevProfile, "development"), devDefaults)
		if err != nil {
			return err
		}
	} else {
		project.DevProfile = project.ProdProfile
	}
	if profile, ok := userConfig.Profiles[project.ProdProfile]; ok && project.Realm == "" {
		project.Realm = profile.Realm
	}
	if project.Realm == "" {
		return errors.New("a Quickbase realm is required")
	}

	if err := deployqb.SaveProject(deployqb.ProjectFilename, project); err != nil {
		return err
	}
	personal := userConfig.Personal
	entry := personal[project.RepositoryID]
	if entry.FeaturePrefix == "" {
		entry.FeaturePrefix = envOr("CUSTOM_PREFIX_FEATURE", "F")
	}
	personal[project.RepositoryID] = entry
	userConfig.Personal = personal
	if err := deployqb.SaveUserConfig(userConfigFilename, userConfig); err != nil {
		return err
	}
	fmt.Fprintln(writer, "Initialized qbcli.json. Update filesConf with the files to deploy.")
	return nil
}

func initialProject() (deployqb.Project, error) {
	project, err := deployqb.LoadProject(deployqb.ProjectFilename)
	if err == nil {
		return project, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return project, err
	}
	return deployqb.Project{Files: []deployqb.FileConfig{}}, nil
}

func configureProfile(prompter *initPrompter, profiles map[string]deployqb.Profile, role, name string, defaults deployqb.Profile) (string, error) {
	if profiles == nil {
		profiles = map[string]deployqb.Profile{}
	}
	names := make([]string, 0, len(profiles))
	for profileName := range profiles {
		names = append(names, profileName)
	}
	sort.Strings(names)
	if len(names) > 0 {
		fmt.Fprintf(prompter.writer, "Existing profiles: %s\n", strings.Join(names, ", "))
	}
	selected, err := prompter.ask(role+" profile name", name)
	if err != nil {
		return "", err
	}
	if selected == "" {
		return "", fmt.Errorf("%s profile name is required", strings.ToLower(role))
	}
	profile := defaults
	if existing, ok := profiles[selected]; ok {
		profile = existing
	}
	if profile.Realm, err = prompter.ask(role+" profile realm", profile.Realm); err != nil {
		return "", err
	}
	if profile.UserToken, err = prompter.askSecret(role+" user token", profile.UserToken); err != nil {
		return "", err
	}
	if profile.AppToken, err = prompter.askSecret(role+" app token", profile.AppToken); err != nil {
		return "", err
	}
	if profile.UserToken == "" {
		if prompter.terminal != nil {
			return "", fmt.Errorf("%s user token is required", strings.ToLower(role))
		}
		// Keep old noninteractive setup usable when credentials are supplied later
		// through the process environment.
		return "", nil
	}
	if profile.Realm == "" {
		return "", fmt.Errorf("%s profile realm is required", strings.ToLower(role))
	}
	profiles[selected] = profile
	return selected, nil
}

func defaultProfile(profiles map[string]deployqb.Profile, fallback string) string {
	if len(profiles) != 1 {
		return fallback
	}
	for name := range profiles {
		return name
	}
	return fallback
}

func yesNo(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "y", "yes", "true":
		return "yes"
	case "n", "no", "false":
		return "no"
	default:
		return value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func deploy(environment string) error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	if len(project.Files) == 0 {
		return errors.New("qbcli.json must list at least one file in filesConf")
	}
	userConfig, userConfigFilename, err := loadUserConfig()
	if err != nil {
		return err
	}
	personal := userConfig.Personal
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
	if separateDevApp {
		dbid = project.DevDBID
	}
	realm, userToken, appToken, err := credentials(project, userConfig, separateDevApp)
	if err != nil {
		return err
	}
	if dbid == "" || realm == "" || userToken == "" {
		return errors.New("selected environment is missing a DBID, realm, or user token")
	}

	client := quickbase.Client{}
	for _, file := range prepared {
		pageID, err := client.AddReplaceDBPage(dbid, realm, userToken, appToken, file.Name, file.Body)
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
	userConfig.Personal = personal
	if err := deployqb.SaveUserConfig(userConfigFilename, userConfig); err != nil {
		return err
	}
	fmt.Printf("Deployed %d files to %s.\n", len(prepared), environment)
	return nil
}

func credentials(project deployqb.Project, config deployqb.UserConfig, useDevelopment bool) (realm, userToken, appToken string, err error) {
	profileName := project.ProdProfile
	if useDevelopment {
		profileName = project.DevProfile
	}
	if profileName != "" {
		profile, ok := config.Profiles[profileName]
		if !ok {
			return "", "", "", fmt.Errorf("profile %q is not available; run deployqb init", profileName)
		}
		return profile.Realm, profile.UserToken, profile.AppToken, nil
	}
	realm = firstNonEmpty(project.Realm, envOr("QB_REALM", os.Getenv("REALM")))
	if useDevelopment {
		userToken = firstNonEmpty(userToken, envOr("QB_DEV_USERTOKEN", envOr("DEV_USERTOKEN", envOr("QB_USERTOKEN", os.Getenv("USERTOKEN")))))
		appToken = firstNonEmpty(appToken, envOr("QB_DEV_APP_TOKEN", envOr("DEV_APP_TOKEN", envOr("QB_APP_TOKEN", os.Getenv("APP_TOKEN")))))
	} else {
		userToken = firstNonEmpty(userToken, envOr("QB_USERTOKEN", os.Getenv("USERTOKEN")))
		appToken = firstNonEmpty(appToken, envOr("QB_APP_TOKEN", os.Getenv("APP_TOKEN")))
	}
	return realm, userToken, appToken, nil
}

func launch(environment string) error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	userConfig, _, err := loadUserConfig()
	if err != nil {
		return err
	}
	dbid, pageID := project.DBID, project.LaunchProdPageID
	useDevelopment := project.DevAndProdQuickBaseApplications == "yes" && environment != "prod"
	if environment != "prod" {
		if project.DevAndProdQuickBaseApplications == "yes" {
			dbid = project.DevDBID
		}
		if environment == "dev" {
			pageID = project.LaunchDevPageID
		} else {
			pageID = userConfig.Personal[project.RepositoryID].LaunchFeatPageID
		}
	}
	if pageID == 0 {
		return fmt.Errorf("deploy %s with an isIndexFile entry before launching it", environment)
	}
	realm, _, _, err := credentials(project, userConfig, useDevelopment)
	if err != nil {
		return err
	}
	address, err := launchAddress(realm, dbid, pageID, project.URLQueryString)
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
	userConfig, filename, err := loadUserConfig()
	if err != nil {
		return err
	}
	entry, ok := userConfig.Personal[project.RepositoryID]
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
	userConfig.Personal[project.RepositoryID] = entry
	return deployqb.SaveUserConfig(filename, userConfig)
}

func generateLinks() error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	userConfig, _, err := loadUserConfig()
	if err != nil {
		return err
	}
	for _, environment := range []string{"dev", "prod", "feat"} {
		prefix, err := deployqb.Prefix(project, userConfig.Personal[project.RepositoryID], environment)
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

func loadUserConfig() (deployqb.UserConfig, string, error) {
	filename, err := deployqb.PersonalConfigPath()
	if err != nil {
		return deployqb.UserConfig{}, "", err
	}
	config, err := deployqb.LoadUserConfig(filename)
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
