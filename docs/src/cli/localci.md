# CLI Reference

## localci

Local post-commit validation runner

### Synopsis

localci is a local post-commit validation runner.

Commands that operate on a repo default to the nearest ancestor of the current directory that contains .git.

```
localci [flags]
```

### Options

```
  -h, --help      help for localci
  -v, --version   version for localci
```

### SEE ALSO

* [localci artifacts](../localci_artifacts/)	 - Print filesystem paths for task artifacts
* [localci cancel](../localci_cancel/)	 - Cancel queued or active work
* [localci cat](../localci_cat/)	 - Print one artifact to stdout
* [localci completion](../localci_completion/)	 - Generate the autocompletion script for the specified shell
* [localci dash](../localci_dash/)	 - Open the terminal UI
* [localci docs](../localci_docs/)	 - Run this to quickly learn everything localci can do
* [localci history](../localci_history/)	 - Print recent runs
* [localci install-hooks](../localci_install-hooks/)	 - Install the Git post-commit hook
* [localci invoke](../localci_invoke/)	 - Run an ad hoc check directly
* [localci postcommit](../localci_postcommit/)	 - Enqueue tasks for a committed revision
* [localci restart](../localci_restart/)	 - Restart the daemon
* [localci run](../localci_run/)	 - Queue a daemon-managed run manually
* [localci start](../localci_start/)	 - Start the daemon
* [localci status](../localci_status/)	 - Print a bounded status summary
* [localci stop](../localci_stop/)	 - Stop the daemon
* [localci wait](../localci_wait/)	 - Wait for a selected run or task to complete
* [localci web](../localci_web/)	 - Open the web UI

