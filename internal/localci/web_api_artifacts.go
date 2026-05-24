package localci

import (
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
)

func (s WebServer) handleAPIArtifactIndex(w http.ResponseWriter, repoDir string, commit string, taskName string, attempt int) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, apiArtifactListResponse{
		Repo:      s.apiRepoSummary(repoDir),
		Commit:    commit,
		Task:      task.Name,
		Attempt:   task.Attempt,
		Artifacts: task.Artifacts,
	})
}

func (s WebServer) handleAPIRevealArtifact(w http.ResponseWriter, repoDir string, commit string, taskName string, attempt int, artifactPath string) {
	_, artifact, err := s.artifactForRoute(repoDir, commit, taskName, attempt, artifactPath)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	reveal := s.RevealArtifactInFinder
	if reveal == nil {
		reveal = defaultRevealArtifactInFinder
	}
	if err := reveal(artifact.Path); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiRevealArtifactResponse{Path: artifact.Path, OK: true})
}

func (s WebServer) handleAPIArtifact(w http.ResponseWriter, repoDir string, commit string, taskName string, attempt int, artifactPath string) {
	task, artifact, err := s.artifactForRoute(repoDir, commit, taskName, attempt, artifactPath)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	data, err := readTextTaskArtifact(task, artifact.DisplayName)
	if err != nil {
		if errors.Is(err, ErrArtifactNotDisplayable) {
			writeAPIError(w, http.StatusUnsupportedMediaType, err)
			return
		}
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, apiArtifactResponse{
		Repo:     s.apiRepoSummary(repoDir),
		Commit:   commit,
		Task:     task.Name,
		Attempt:  task.Attempt,
		Artifact: artifact,
		Content:  string(data),
	})
}

func (s WebServer) artifactForRoute(repoDir string, commit string, taskName string, attempt int, artifactPath string) (TaskStatusView, ArtifactView, error) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		return TaskStatusView{}, ArtifactView{}, err
	}
	artifact, ok := findArtifactByDisplayName(task.Artifacts, artifactPath)
	if !ok {
		return TaskStatusView{}, ArtifactView{}, ErrRecordNotFound
	}
	return task, artifact, nil
}

func defaultRevealArtifactInFinder(artifactPath string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("show in finder is only supported on macOS")
	}
	return exec.Command("open", "-R", artifactPath).Run()
}

func findArtifactByDisplayName(artifacts []ArtifactView, displayName string) (ArtifactView, bool) {
	for _, artifact := range artifacts {
		if artifact.DisplayName == displayName {
			return artifact, true
		}
	}
	return ArtifactView{}, false
}
