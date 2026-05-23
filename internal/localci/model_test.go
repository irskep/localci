package localci

import "testing"

func TestTrimTaskPrefix(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"localci:test":       "test",
		"//:localci:build":   "build",
		"//web:localci:lint": "//web:lint",
		"//web:deploy":       "//web:deploy",
		"not-a-localci-task": "not-a-localci-task",
	}

	for input, want := range tests {
		if got := trimTaskPrefix(input); got != want {
			t.Fatalf("trimTaskPrefix(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsRootSetupTask(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"localci:setup":       true,
		"//:localci:setup":    true,
		"//web:localci:setup": false,
		"localci:test":        false,
	}

	for input, want := range tests {
		if got := isRootSetupTask(input); got != want {
			t.Fatalf("isRootSetupTask(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestSplitSetupTaskRunsOnlyRootSetup(t *testing.T) {
	t.Parallel()

	setup, tasks, ok := splitSetupTask([]Task{
		{Name: "//docs:localci:setup"},
		{Name: "//:localci:setup"},
		{Name: "//web:localci:test"},
		{Name: "localci:test"},
	})
	if !ok {
		t.Fatalf("splitSetupTask did not find root setup")
	}
	if setup.Name != "//:localci:setup" {
		t.Fatalf("setup = %q, want root setup", setup.Name)
	}
	if len(tasks) != 2 || tasks[0].Name != "//web:localci:test" || tasks[1].Name != "localci:test" {
		t.Fatalf("tasks = %#v, want setup tasks removed", tasks)
	}
}

func TestSplitSetupTaskIgnoresChildSetupWithoutRootSetup(t *testing.T) {
	t.Parallel()

	_, tasks, ok := splitSetupTask([]Task{
		{Name: "//docs:localci:setup"},
		{Name: "//web:localci:test"},
	})
	if ok {
		t.Fatalf("splitSetupTask found setup, want no root setup")
	}
	if len(tasks) != 1 || tasks[0].Name != "//web:localci:test" {
		t.Fatalf("tasks = %#v, want child setup removed", tasks)
	}
}
