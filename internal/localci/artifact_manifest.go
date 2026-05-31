package localci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const taskArtifactsManifestName = "localci.artifacts.json"

type taskArtifactsManifest struct {
	Version   int              `json:"version"`
	Artifacts []MarkedArtifact `json:"artifacts"`
}

func taskArtifactsManifestPath(outputDir string) string {
	return filepath.Join(outputDir, taskArtifactsManifestName)
}

func readTaskArtifactsManifest(outputDir string) ([]MarkedArtifact, error) {
	path := taskArtifactsManifestPath(outputDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read task artifacts manifest: %w", err)
	}

	var manifest taskArtifactsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse task artifacts manifest: %w", err)
	}
	if manifest.Version != 1 {
		return nil, fmt.Errorf("task artifacts manifest version must be 1")
	}

	seenNames := map[string]bool{}
	seenPaths := map[string]bool{}
	marked := make([]MarkedArtifact, 0, len(manifest.Artifacts))
	for index, artifact := range manifest.Artifacts {
		artifact.Name = strings.TrimSpace(artifact.Name)
		artifact.Path = filepath.ToSlash(strings.TrimSpace(artifact.Path))
		if artifact.Name == "" {
			return nil, fmt.Errorf("task artifacts manifest artifact %d name is required", index)
		}
		if seenNames[artifact.Name] {
			return nil, fmt.Errorf("task artifacts manifest artifact name %q is duplicated", artifact.Name)
		}
		seenNames[artifact.Name] = true
		if artifact.Path == "" {
			return nil, fmt.Errorf("task artifacts manifest artifact %q path is required", artifact.Name)
		}
		if filepath.IsAbs(filepath.FromSlash(artifact.Path)) {
			return nil, fmt.Errorf("task artifacts manifest artifact %q path must be relative", artifact.Name)
		}
		cleanPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(artifact.Path)))
		if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
			return nil, fmt.Errorf("task artifacts manifest artifact %q path must stay inside output dir", artifact.Name)
		}
		artifact.Path = cleanPath
		if seenPaths[artifact.Path] {
			return nil, fmt.Errorf("task artifacts manifest artifact path %q is duplicated", artifact.Path)
		}
		seenPaths[artifact.Path] = true
		if !validArtifactAction(artifact.Action) {
			return nil, fmt.Errorf("task artifacts manifest artifact %q action %q is invalid", artifact.Name, artifact.Action)
		}
		fullPath := filepath.Join(outputDir, filepath.FromSlash(artifact.Path))
		if _, err := os.Stat(fullPath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("task artifacts manifest artifact %q path does not exist: %s", artifact.Name, artifact.Path)
			}
			return nil, fmt.Errorf("stat task artifacts manifest artifact %q: %w", artifact.Name, err)
		}
		marked = append(marked, artifact)
	}
	return marked, nil
}

func validArtifactAction(action ArtifactAction) bool {
	switch action {
	case ArtifactActionOpen, ArtifactActionDownload, ArtifactActionReveal, ArtifactActionView:
		return true
	default:
		return false
	}
}
