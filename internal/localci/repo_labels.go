package localci

import (
	"path/filepath"
	"sort"
	"strings"
)

func RepoLabelMap(repoDirs []string) map[string]string {
	cleaned := make([]string, 0, len(repoDirs))
	seen := map[string]bool{}
	for _, repoDir := range repoDirs {
		clean := filepath.Clean(repoDir)
		if clean == "." || clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		cleaned = append(cleaned, clean)
	}

	labels := make(map[string]string, len(cleaned))
	if len(cleaned) == 0 {
		return labels
	}
	if len(cleaned) == 1 {
		labels[cleaned[0]] = filepath.Base(cleaned[0])
		return labels
	}

	common := commonPathComponents(cleaned)
	for _, repoDir := range cleaned {
		components := pathComponents(repoDir)
		if len(common) < len(components) {
			labels[repoDir] = filepath.ToSlash(filepath.Join(components[len(common):]...))
			continue
		}
		labels[repoDir] = filepath.Base(repoDir)
	}

	ensureUniqueLabels(labels, cleaned)
	return labels
}

func RepoDisplayLabel(repoDir string, allRepoDirs []string) string {
	labels := RepoLabelMap(append(allRepoDirs, repoDir))
	if label := strings.TrimSpace(labels[filepath.Clean(repoDir)]); label != "" {
		return label
	}
	return filepath.Base(filepath.Clean(repoDir))
}

func commonPathComponents(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	common := pathComponents(paths[0])
	for _, path := range paths[1:] {
		components := pathComponents(path)
		limit := min(len(common), len(components))
		index := 0
		for index < limit && common[index] == components[index] {
			index++
		}
		common = common[:index]
		if len(common) == 0 {
			break
		}
	}
	return common
}

func ensureUniqueLabels(labels map[string]string, repoDirs []string) {
	for {
		byLabel := map[string][]string{}
		for _, repoDir := range repoDirs {
			byLabel[labels[repoDir]] = append(byLabel[labels[repoDir]], repoDir)
		}

		changed := false
		for _, duplicates := range byLabel {
			if len(duplicates) < 2 {
				continue
			}
			sort.Strings(duplicates)
			for _, repoDir := range duplicates {
				labels[repoDir] = extendSuffixLabel(repoDir, labels[repoDir])
			}
			changed = true
		}
		if !changed {
			return
		}
	}
}

func extendSuffixLabel(repoDir string, label string) string {
	components := pathComponents(repoDir)
	labelParts := strings.Split(filepath.FromSlash(label), string(filepath.Separator))
	if len(labelParts) >= len(components) {
		return filepath.ToSlash(filepath.Join(components...))
	}
	start := len(components) - len(labelParts) - 1
	if start < 0 {
		start = 0
	}
	return filepath.ToSlash(filepath.Join(components[start:]...))
}

func pathComponents(path string) []string {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	rest := strings.TrimPrefix(clean, volume)
	rest = strings.Trim(rest, string(filepath.Separator))

	parts := make([]string, 0)
	if volume != "" {
		parts = append(parts, volume)
	}
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
