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
	"strings"
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
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiArtifactListResponse{
		Repo:      repo,
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
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiArtifactResponse{
		Repo:     repo,
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
	route, err := s.parseRawArtifactRoute(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	task, artifact, err := s.artifactForRoute(route.RepoDir, route.Commit, route.TaskName, route.Attempt, route.ArtifactPath)
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

type rawArtifactRoute struct {
	RepoDir      string
	Commit       string
	TaskName     string
	Attempt      int
	ArtifactPath string
}

func (s WebServer) parseRawArtifactRoute(r *http.Request) (rawArtifactRoute, error) {
	escapedPath := requestEscapedPath(r)
	segments, err := splitEscapedPath(escapedPath)
	if err != nil {
		return rawArtifactRoute{}, err
	}
	if len(segments) < 10 || segments[0] != "artifacts" || segments[1] != "repo" {
		return rawArtifactRoute{}, fmt.Errorf("unsupported artifact route")
	}
	commitIndex := indexOfSegment(segments[2:], "commit")
	if commitIndex < 0 {
		return rawArtifactRoute{}, fmt.Errorf("artifact route missing commit")
	}
	commitIndex += 2
	repoDir, err := s.repoDirFromRoute(segments[2:commitIndex])
	if err != nil {
		return rawArtifactRoute{}, err
	}
	tail := segments[commitIndex:]
	if len(tail) < 7 || tail[0] != "commit" || tail[2] != "task" || tail[4] != "attempt" {
		return rawArtifactRoute{}, fmt.Errorf("unsupported artifact route")
	}
	attempt, err := strconv.Atoi(tail[5])
	if err != nil || attempt <= 0 {
		return rawArtifactRoute{}, fmt.Errorf("attempt must be a positive integer")
	}
	artifactPath, err := rawArtifactDisplayName(tail[6:], strings.HasSuffix(escapedPath, "/"))
	if err != nil {
		return rawArtifactRoute{}, err
	}
	return rawArtifactRoute{
		RepoDir:      repoDir,
		Commit:       tail[1],
		TaskName:     tail[3],
		Attempt:      attempt,
		ArtifactPath: artifactPath,
	}, nil
}

func rawArtifactDisplayName(segments []string, trailingSlash bool) (string, error) {
	artifactPath := path.Join(segments...)
	if artifactPath == "" || artifactPath == "." {
		return "", fmt.Errorf("artifact path is required")
	}
	if trailingSlash {
		artifactPath = path.Join(artifactPath, "index.html")
	}
	return artifactPath, nil
}

func (s WebServer) enrichTaskArtifacts(repoDir string, commit string, task TaskStatusView) TaskStatusView {
	for index := range task.Artifacts {
		task.Artifacts[index] = s.enrichArtifact(repoDir, commit, task, task.Artifacts[index])
	}
	return task
}

func (s WebServer) enrichMarkedArtifacts(repoDir string, commit string, task TaskStatusView) []ArtifactView {
	task = s.enrichTaskArtifacts(repoDir, commit, task)
	return markedArtifactViews(task)
}

func (s WebServer) markedArtifactViewsFromRecord(repoDir string, commit string, record TaskRecord) []ArtifactView {
	artifacts := make([]ArtifactView, 0, len(record.MarkedArtifacts))
	for _, marked := range record.MarkedArtifacts {
		artifact := ArtifactView{
			Name:        filepath.Base(filepath.FromSlash(marked.Path)),
			DisplayName: marked.Path,
			Path:        filepath.Join(record.OutputDir, filepath.FromSlash(marked.Path)),
			MarkedName:  marked.Name,
			Action:      marked.Action,
		}
		if rawURL, err := RawArtifactRoutePath(repoDir, commit, record.Name, record.Attempt, artifact.DisplayName); err == nil {
			artifact.RawURL = rawURL
			artifact.DownloadURL = rawURL + "?download=1"
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func markedArtifactViews(task TaskStatusView) []ArtifactView {
	artifacts := []ArtifactView{}
	for _, artifact := range task.Artifacts {
		if artifact.MarkedName != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts
}

func (s WebServer) enrichArtifact(repoDir string, commit string, task TaskStatusView, artifact ArtifactView) ArtifactView {
	_, textErr := readTextTaskArtifact(task, artifact.DisplayName)
	artifact.IsText = textErr == nil
	if rawURL, err := RawArtifactRoutePath(repoDir, commit, task.Name, task.Attempt, artifact.DisplayName); err == nil {
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
