package localci

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
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
	task = s.enrichTaskArtifacts(repoDir, commit, task)
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
			data = nil
		} else {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
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
	task = s.enrichTaskArtifacts(repoDir, commit, task)
	artifact, ok := findArtifactByDisplayName(task.Artifacts, artifactPath)
	if !ok {
		return TaskStatusView{}, ArtifactView{}, ErrRecordNotFound
	}
	return task, artifact, nil
}

func (s WebServer) handleRawArtifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	repoDir, commit, taskName, attempt, artifactPath, err := s.parseRawArtifactRoute(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	task, artifact, err := s.artifactForRoute(repoDir, commit, taskName, attempt, artifactPath)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			http.NotFound(w, r)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if shouldHideArtifact(artifact.DisplayName) {
		http.NotFound(w, r)
		return
	}
	resolvedPath, err := resolveArtifactPath(task.OutputDir, artifact.DisplayName)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
			"filename": filepath.Base(artifact.DisplayName),
		}))
	}
	if filepath.Base(artifact.DisplayName) == "index.html" {
		serveRawArtifactContent(w, r, resolvedPath)
		return
	}
	const fileServerPrefix = "/__localci_artifact__/"
	fileRequest := r.Clone(r.Context())
	fileRequest.URL.Path = fileServerPrefix + filepath.ToSlash(artifact.DisplayName)
	fileRequest.URL.RawPath = ""
	fileRequest.RequestURI = ""
	http.StripPrefix(fileServerPrefix, http.FileServer(http.Dir(task.OutputDir))).ServeHTTP(w, fileRequest)
}

func serveRawArtifactContent(w http.ResponseWriter, r *http.Request, artifactPath string) {
	file, err := os.Open(artifactPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (s WebServer) parseRawArtifactRoute(r *http.Request) (string, string, string, int, string, error) {
	segments, err := splitEscapedPath(requestEscapedPath(r))
	if err != nil {
		return "", "", "", 0, "", err
	}
	if len(segments) < 10 || segments[0] != "artifacts" || segments[1] != "repo" {
		return "", "", "", 0, "", fmt.Errorf("unsupported artifact route")
	}
	commitIndex := indexOfSegment(segments[2:], "commit")
	if commitIndex < 0 {
		return "", "", "", 0, "", fmt.Errorf("artifact route missing commit")
	}
	commitIndex += 2
	repoDir, err := s.repoDirFromRoute(segments[2:commitIndex])
	if err != nil {
		return "", "", "", 0, "", err
	}
	tail := segments[commitIndex:]
	if len(tail) < 7 || tail[0] != "commit" || tail[2] != "task" || tail[4] != "attempt" {
		return "", "", "", 0, "", fmt.Errorf("unsupported artifact route")
	}
	attempt, err := strconv.Atoi(tail[5])
	if err != nil || attempt <= 0 {
		return "", "", "", 0, "", fmt.Errorf("attempt must be a positive integer")
	}
	artifactPath := path.Join(tail[6:]...)
	if artifactPath == "" || artifactPath == "." {
		return "", "", "", 0, "", fmt.Errorf("artifact path is required")
	}
	return repoDir, tail[1], tail[3], attempt, artifactPath, nil
}

func (s WebServer) enrichTaskArtifacts(repoDir string, commit string, task TaskStatusView) TaskStatusView {
	for index := range task.Artifacts {
		task.Artifacts[index] = s.enrichArtifact(repoDir, commit, task, task.Artifacts[index])
	}
	return task
}

func (s WebServer) enrichArtifact(repoDir string, commit string, task TaskStatusView, artifact ArtifactView) ArtifactView {
	_, textErr := readTextTaskArtifact(task, artifact.DisplayName)
	artifact.IsText = textErr == nil
	if rawURL, err := RawArtifactRoutePath(s.configuredRepoRoot(), repoDir, commit, task.Name, task.Attempt, artifact.DisplayName); err == nil {
		artifact.RawURL = rawURL
		artifact.DownloadURL = rawURL + "?download=1"
	}
	return artifact
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
