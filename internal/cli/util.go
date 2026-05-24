package cli

import (
	"os"
	"strings"
)

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}

	return wd
}

func pluralizeTask(count int) string {
	if count == 1 {
		return "task"
	}
	return "tasks"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!#&*()[]{}|;<>?~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
