package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	managementGroupID = "management"
	startingGroupID   = "starting"
	uiGroupID         = "uis"
	viewingGroupID    = "viewing"
)

func (a App) newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "localci",
		Short:         "Local post-commit validation runner",
		Long:          "localci is a local post-commit validation runner.\n\nCommands that operate on a repo default to the nearest ancestor of the current directory that contains .git.",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("unexpected positional arguments: %s", strings.Join(args, " "))
			}
			return cmd.Help()
		},
	}
	cmd.SetOut(a.Stdout)
	cmd.SetErr(a.Stderr)
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.SetCompletionCommandGroupID(managementGroupID)
	cmd.AddGroup(
		&cobra.Group{ID: managementGroupID, Title: "Management"},
		&cobra.Group{ID: startingGroupID, Title: "Starting Tasks"},
		&cobra.Group{ID: uiGroupID, Title: "UIs"},
		&cobra.Group{ID: viewingGroupID, Title: "Viewing Status/History"},
	)

	cmd.AddCommand(
		a.newStartCommand(),
		a.newRestartCommand(),
		a.newStopCommand(),
		a.newDaemonCommand(),
		a.newPostcommitCommand(),
		a.newRunCommand(),
		a.newInvokeCommand(),
		a.newCancelCommand(),
		a.newWaitCommand(),
		a.newStatusCommand(),
		a.newHistoryCommand(),
		a.newArtifactsCommand(),
		a.newDocsCommand(),
		a.newWebCommand(),
		a.newDashCommand(),
		a.newInstallHooksCommand(),
	)
	return cmd
}

func (a App) runCommand(groupID string, use string, short string, long string, example string, configure func(*cobra.Command, *cliFlags, *[]string), run func(cliFlags) error) *cobra.Command {
	flags := cliFlags{Limit: 20}
	var annotations []string
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		GroupID: groupID,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := finishCLIFlags(cmd, &flags, annotations); err != nil {
				return err
			}
			if err := a.checkRequirements(); err != nil {
				return err
			}
			return run(flags)
		},
	}
	if configure != nil {
		configure(cmd, &flags, &annotations)
	}
	return cmd
}

func (a App) noArgCommand(groupID string, use string, short string, long string, run func() error) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		GroupID: groupID,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.checkRequirements(); err != nil {
				return err
			}
			return run()
		},
	}
}

func (a App) newStartCommand() *cobra.Command {
	return a.noArgCommand(managementGroupID, "start", "Start the daemon", "Start the LocalCI daemon.", a.runStart)
}

func (a App) newRestartCommand() *cobra.Command {
	return a.noArgCommand(managementGroupID, "restart", "Restart the daemon", "Restart the LocalCI daemon.", a.runRestart)
}

func (a App) newStopCommand() *cobra.Command {
	return a.noArgCommand(managementGroupID, "stop", "Stop the daemon", "Stop the LocalCI daemon.", a.runStop)
}

func (a App) newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "daemon",
		Short:   "Run daemon internals",
		GroupID: managementGroupID,
		Hidden:  true,
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(a.noArgCommand("", "run", "Run the daemon in the foreground", "Run the LocalCI daemon in the foreground.", a.runDaemon))
	return cmd
}

func (a App) newPostcommitCommand() *cobra.Command {
	return a.runCommand(
		startingGroupID,
		"postcommit",
		"Enqueue tasks for a committed revision",
		"Enqueue tasks for a committed revision. This is the command the Git post-commit hook calls.",
		"localci postcommit --commit HEAD\nlocalci postcommit --commit HEAD --task test\nlocalci postcommit --repo \"$repo\" --commit \"$commit\"",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addAnnotationFlag(cmd, annotations)
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runPostcommit,
	)
}

func (a App) newRunCommand() *cobra.Command {
	return a.runCommand(
		startingGroupID,
		"run",
		"Queue a daemon-managed run manually",
		"Queue a daemon-managed run manually. Use --no-clone to run against the live working tree.",
		"localci run --wait\nlocalci run --no-clone --wait\nlocalci run --no-clone --task test --wait",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addNoCloneFlag(cmd, flags)
			cmd.Flags().BoolVar(&flags.Wait, "wait", false, "wait for the selected run to complete")
			addAnnotationFlag(cmd, annotations)
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runRun,
	)
}

func (a App) newInvokeCommand() *cobra.Command {
	return a.runCommand(
		startingGroupID,
		"invoke",
		"Run an ad hoc check directly",
		"Run an ad hoc check directly in the current terminal. Use run when you want daemon-managed queueing.",
		"localci invoke --task test --wait\nlocalci invoke --no-clone --task test --wait\nlocalci invoke --commit HEAD --annotation branch=main",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addNoCloneFlag(cmd, flags)
			cmd.Flags().BoolVar(&flags.Wait, "wait", false, "wait for the selected run to complete")
			addAnnotationFlag(cmd, annotations)
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runInvoke,
	)
}

func (a App) newCancelCommand() *cobra.Command {
	return a.runCommand(
		viewingGroupID,
		"cancel",
		"Cancel queued or active work",
		"Cancel queued or active LocalCI work.",
		"localci cancel\nlocalci cancel --task noisy-fail\nlocalci cancel --commit 'HEAD*' --task test",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addNoCloneFlag(cmd, flags)
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runCancel,
	)
}

func (a App) newWaitCommand() *cobra.Command {
	return a.runCommand(
		viewingGroupID,
		"wait",
		"Wait for a selected run or task to complete",
		"Wait for the selected run or task to complete and show the result.",
		"localci wait\nlocalci wait --task noisy-fail\nlocalci wait --commit 'HEAD*'",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addNoCloneFlag(cmd, flags)
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runWait,
	)
}

func (a App) newStatusCommand() *cobra.Command {
	return a.runCommand(
		viewingGroupID,
		"status",
		"Print a bounded status summary",
		"Print a bounded status summary for the selected run.",
		"localci status\nlocalci status --task noisy-fail\nlocalci status --commit HEAD\nlocalci status --commit 'HEAD*' --task test",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addAttemptFlag(cmd, flags)
			addNoCloneFlag(cmd, flags)
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runStatus,
	)
}

func (a App) newHistoryCommand() *cobra.Command {
	return a.runCommand(
		viewingGroupID,
		"history",
		"Print recent runs",
		"Print recent LocalCI runs.",
		"localci history\nlocalci history --task noisy-fail\nlocalci history --failed\nlocalci history --task //web:localci:test --limit 50",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			cmd.Flags().StringArrayVar(&flags.Statuses, "status", nil, "filter by status")
			cmd.Flags().BoolVar(&flags.Failed, "failed", false, "show failed and timed-out tasks")
			cmd.Flags().IntVar(&flags.Limit, "limit", 20, "maximum number of rows to print")
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runHistory,
	)
}

func (a App) newArtifactsCommand() *cobra.Command {
	return a.runCommand(
		viewingGroupID,
		"artifacts",
		"Print filesystem paths for task artifacts",
		"Print filesystem paths for task artifacts.",
		"localci artifacts\nlocalci artifacts --task noisy-fail\nlocalci artifacts --failed --primary\nlocalci artifacts --task noisy-fail --primary --paths-only",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addAttemptFlag(cmd, flags)
			cmd.Flags().BoolVar(&flags.Failed, "failed", false, "show artifacts for failed and timed-out tasks")
			cmd.Flags().BoolVar(&flags.Primary, "primary", false, "show only primary artifacts")
			cmd.Flags().BoolVar(&flags.PathsOnly, "paths-only", false, "print only artifact paths")
			cmd.Flags().BoolVar(&flags.JSON, "json", false, "print JSON output")
		},
		a.runArtifacts,
	)
}

func (a App) newDocsCommand() *cobra.Command {
	var plain bool
	var roff bool
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Run this to quickly learn everything localci can do",
		Long:  "Read LocalCI's bundled narrative documentation. For command options, use localci --help or localci <command> --help.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if plain && roff {
				return fmt.Errorf("--plain and --roff are mutually exclusive")
			}
			return a.runDocs(docsFlags{Plain: plain, Roff: roff})
		},
	}
	cmd.Flags().BoolVar(&plain, "plain", false, "print plain text instead of opening a man-style viewer")
	cmd.Flags().BoolVar(&roff, "roff", false, "print roff manpage source")
	return cmd
}

func (a App) newWebCommand() *cobra.Command {
	return a.runCommand(
		uiGroupID,
		"web",
		"Open the web UI",
		"Open the web UI. It shows comprehensive information including logs, live status, downloadable binaries, and HTML task output.",
		"localci web\nlocalci web --task noisy-fail\nlocalci web --task noisy-fail --artifact combined.log",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addAttemptFlag(cmd, flags)
			cmd.Flags().StringVar(&flags.Artifact, "artifact", "", "artifact display name")
			addNoCloneFlag(cmd, flags)
		},
		a.runWeb,
	)
}

func (a App) newDashCommand() *cobra.Command {
	return a.runCommand(
		uiGroupID,
		"dash",
		"Open the terminal UI",
		"Open the terminal UI. It uses the daemon's REST and websocket APIs and defaults to the all-repos home view when no target is provided.",
		"localci dash\nlocalci dash --task noisy-fail",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			addRepoCommitTaskFlags(cmd, flags)
			addAttemptFlag(cmd, flags)
			cmd.Flags().StringVar(&flags.Artifact, "artifact", "", "artifact display name")
			addNoCloneFlag(cmd, flags)
		},
		a.runDash,
	)
}

func (a App) newInstallHooksCommand() *cobra.Command {
	return a.runCommand(
		managementGroupID,
		"install-hooks",
		"Install the Git post-commit hook",
		"Install LocalCI's Git post-commit hook entry for a repository.",
		"localci install-hooks\nlocalci install-hooks --repo /path/to/repo",
		func(cmd *cobra.Command, flags *cliFlags, annotations *[]string) {
			cmd.Flags().StringVar(&flags.Repo, "repo", "", "repo directory; defaults to nearest Git repo ancestor")
		},
		a.runInstallHooks,
	)
}

func addRepoCommitTaskFlags(cmd *cobra.Command, flags *cliFlags) {
	cmd.Flags().StringVar(&flags.Repo, "repo", "", "repo directory; defaults to nearest Git repo ancestor")
	cmd.Flags().StringVar(&flags.Commit, "commit", "", "commit ref; defaults depend on command")
	cmd.Flags().StringVar(&flags.Task, "task", "", "full task name or unambiguous short task name")
}

func addAttemptFlag(cmd *cobra.Command, flags *cliFlags) {
	cmd.Flags().IntVar(&flags.Attempt, "attempt", 0, "attempt number; defaults to latest attempt")
}

func addNoCloneFlag(cmd *cobra.Command, flags *cliFlags) {
	cmd.Flags().BoolVar(&flags.NoClone, "no-clone", false, "run against or select the working tree instead of an isolated clone")
}

func addAnnotationFlag(cmd *cobra.Command, annotations *[]string) {
	cmd.Flags().StringArrayVar(annotations, "annotation", nil, "attach a run annotation as key=value")
}

func finishCLIFlags(cmd *cobra.Command, flags *cliFlags, annotations []string) error {
	if cmd.Flags().Changed("attempt") && flags.Attempt <= 0 {
		return fmt.Errorf("--attempt must be a positive integer")
	}
	if cmd.Flags().Changed("limit") && flags.Limit <= 0 {
		return fmt.Errorf("--limit must be a positive integer")
	}
	if len(annotations) > 0 {
		flags.Annotation = map[string]string{}
	}
	for _, raw := range annotations {
		key, value, ok := strings.Cut(raw, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("--annotation requires key=value")
		}
		flags.Annotation[key] = value
	}
	return nil
}
