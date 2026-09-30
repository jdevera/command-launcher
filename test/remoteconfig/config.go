package remoteconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
)

const (
	baseURLOverrideEnvironment  = "TEST_REMOTE_BASE_URL"
	githubRepositoryEnvironment = "GITHUB_REPOSITORY"
	githubServerURLEnvironment  = "GITHUB_SERVER_URL"
	defaultGitHubServerURL      = "https://github.com"
)

type Config struct {
	DefaultRepository string `json:"default_repository"`
	Tag               string `json:"tag"`
	RegistryPath      string `json:"registry_path"`
}

func Load(filename string) (Config, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("read remote fixture configuration: %w", err)
	}

	var config Config
	if err := json.Unmarshal(contents, &config); err != nil {
		return Config{}, fmt.Errorf("parse remote fixture configuration: %w", err)
	}
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func BaseURL(repositoryRoot string, config Config) (string, error) {
	if err := config.validate(); err != nil {
		return "", err
	}

	if override := os.Getenv(baseURLOverrideEnvironment); override != "" {
		return validateBaseURL(override)
	}

	if repository := os.Getenv(githubRepositoryEnvironment); repository != "" {
		serverURL := os.Getenv(githubServerURLEnvironment)
		if serverURL == "" {
			serverURL = defaultGitHubServerURL
		}
		return buildBaseURL(serverURL, repository, config)
	}

	if serverURL, repository, ok := repositoryFromGitOrigin(repositoryRoot); ok {
		return buildBaseURL(serverURL, repository, config)
	}

	return buildBaseURL(defaultGitHubServerURL, config.DefaultRepository, config)
}

func (config Config) validate() error {
	if !validRepository(config.DefaultRepository) {
		return fmt.Errorf("invalid default remote fixture repository %q", config.DefaultRepository)
	}
	if strings.TrimSpace(config.Tag) == "" {
		return errors.New("remote fixture tag is empty")
	}
	if config.RegistryPath == "" || path.IsAbs(config.RegistryPath) || path.Clean(config.RegistryPath) != config.RegistryPath || strings.HasPrefix(config.RegistryPath, "../") {
		return fmt.Errorf("invalid remote fixture registry path %q", config.RegistryPath)
	}
	return nil
}

func buildBaseURL(serverURL, repository string, config Config) (string, error) {
	serverURL, err := validateServerURL(serverURL)
	if err != nil {
		return "", err
	}
	if !validRepository(repository) {
		return "", fmt.Errorf("invalid remote fixture repository %q", repository)
	}

	return fmt.Sprintf(
		"%s/%s/raw/%s/%s",
		serverURL,
		strings.Trim(repository, "/"),
		url.PathEscape(config.Tag),
		config.RegistryPath,
	), nil
}

func validateBaseURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("invalid %s %q", baseURLOverrideEnvironment, value)
	}
	return strings.TrimRight(value, "/"), nil
}

func validateServerURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path != "" {
		return "", fmt.Errorf("invalid GitHub server URL %q", value)
	}
	return strings.TrimRight(value, "/"), nil
}

func validRepository(value string) bool {
	owner, repository, found := strings.Cut(strings.Trim(value, "/"), "/")
	return found && owner != "" && repository != "" && !strings.Contains(repository, "/")
}

func repositoryFromGitOrigin(repositoryRoot string) (string, string, bool) {
	command := exec.Command("git", "-C", repositoryRoot, "remote", "get-url", "origin")
	output, err := command.Output()
	if err != nil {
		return "", "", false
	}
	return parseGitOrigin(strings.TrimSpace(string(output)))
}

func parseGitOrigin(origin string) (string, string, bool) {
	if before, after, found := strings.Cut(origin, ":"); found && strings.Contains(before, "@") && !strings.Contains(before, "/") {
		host := strings.SplitN(before, "@", 2)[1]
		repository := strings.TrimSuffix(strings.Trim(after, "/"), ".git")
		if host != "" && validRepository(repository) {
			return "https://" + host, repository, true
		}
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() == "" {
		return "", "", false
	}
	repository := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	if !validRepository(repository) {
		return "", "", false
	}
	return "https://" + parsed.Host, repository, true
}
