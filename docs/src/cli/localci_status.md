## localci status

Print a bounded status summary

### Synopsis

Print a bounded status summary for the selected run.

```
localci status [flags]
```

### Examples

```
localci status
localci status --task noisy-fail
localci status --commit HEAD
localci status --commit 'HEAD*' --task test
```

### Options

```
      --attempt int     attempt number; defaults to latest attempt
      --commit string   commit ref; defaults depend on command
  -h, --help            help for status
      --json            print JSON output
      --no-clone        run against or select the working tree instead of an isolated clone
      --repo string     repo directory; defaults to nearest Git repo ancestor
      --task string     full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

