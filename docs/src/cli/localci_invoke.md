## localci invoke

Run an ad hoc check directly

### Synopsis

Run an ad hoc check directly in the current terminal. Use run when you want daemon-managed queueing.

```
localci invoke [flags]
```

### Examples

```
localci invoke --task test --wait
localci invoke --no-clone --task test --wait
localci invoke --commit HEAD --annotation branch=main
```

### Options

```
      --annotation stringArray   attach a run annotation as key=value
      --commit string            commit ref; defaults depend on command
  -h, --help                     help for invoke
      --json                     print JSON output
      --no-clone                 run against or select the working tree instead of an isolated clone
      --repo string              repo directory; defaults to nearest Git repo ancestor
      --task string              full task name or unambiguous short task name
      --wait                     wait for the selected run to complete
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

