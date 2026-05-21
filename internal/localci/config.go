package localci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Root string `toml:"root"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}

	root := strings.TrimSpace(cfg.Root)
	if root == "" {
		return Config{}, fmt.Errorf("config %s: root must not be empty", path)
	}
	if !filepath.IsAbs(root) {
		return Config{}, fmt.Errorf("config %s: root must be absolute", path)
	}

	cfg.Root = filepath.Clean(root)
	return cfg, nil
}

func LoadConfigOrDefault(path string) (Config, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{Root: string(filepath.Separator)}, nil
		}
		return Config{}, err
	}
	return cfg, nil
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".localci", "config.toml"), nil
}

func ResolveRepoDir(root string, repoPath string) (string, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("root must be absolute")
	}

	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return "", fmt.Errorf("repo path must not be empty")
	}

	if filepath.IsAbs(repoPath) {
		abs := filepath.Clean(repoPath)
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return "", err
		}
		if err := validateRelativeRepoPath(rel); err != nil {
			return "", err
		}
		return filepath.Join(root, rel), nil
	}

	if err := validateRelativeRepoPath(repoPath); err != nil {
		return "", err
	}

	return filepath.Join(root, filepath.Clean(repoPath)), nil
}

func validateRelativeRepoPath(path string) error {
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes configured root", path)
	}

	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".." {
			return fmt.Errorf("path %q escapes configured root", path)
		}
	}

	return nil
}
