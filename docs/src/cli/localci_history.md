## localci history

Print recent runs

### Synopsis

Print recent LocalCI runs.

```
localci history [flags]
```

### Examples

```
localci history
localci history --task noisy-fail
localci history --failed
localci history --task //web:localci:test --limit 50
```

### Options

```
      --commit string        commit ref; defaults depend on command
      --failed               show failed and timed-out tasks
  -h, --help                 help for history
      --json                 print JSON output
      --limit int            maximum number of rows to print (default 20)
      --repo string          repo directory; defaults to nearest Git repo ancestor
      --status stringArray   filter by status
      --task string          full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

