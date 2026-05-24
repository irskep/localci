package localci

import (
	"io"
	"os"
	"strings"
	"time"
)

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}

	return time.Now().UTC()
}

func (r Runner) env() []string {
	var env []string
	if len(r.Env) > 0 {
		env = append([]string{}, r.Env...)
	} else {
		env = os.Environ()
	}
	return withEnvVar(env, "MISE_EXPERIMENTAL", "1")
}

func (r Runner) envForDir(dir string) []string {
	env := r.env()
	cleaned := env[:0]
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if key == "PWD" || strings.HasPrefix(key, "__MISE_") {
			continue
		}
		if strings.HasPrefix(key, "MISE_") && key != "MISE_EXPERIMENTAL" {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	return withEnvVar(cleaned, "PWD", dir)
}

func (r Runner) miseBin() string {
	if r.MiseBin != "" {
		return r.MiseBin
	}

	return "mise"
}

func (r Runner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}

	return io.Discard
}

func (r Runner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}

	return io.Discard
}

func (r Runner) inactivityTimeout() time.Duration {
	if r.InactivityTimeout > 0 {
		return r.InactivityTimeout
	}

	return 5 * time.Minute
}

func (r Runner) terminateGrace() time.Duration {
	if r.TerminateGrace > 0 {
		return r.TerminateGrace
	}

	return 5 * time.Second
}

func (r Runner) pollInterval() time.Duration {
	if r.PollInterval > 0 {
		return r.PollInterval
	}

	return time.Second
}

func withEnvVar(env []string, key string, value string) []string {
	prefix := key + "="
	result := append([]string{}, env...)
	for index, entry := range result {
		if strings.HasPrefix(entry, prefix) {
			result[index] = prefix + value
			return result
		}
	}
	return append(result, prefix+value)
}
