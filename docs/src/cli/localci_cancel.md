## localci cancel

Cancel queued or active work

### Synopsis

Cancel queued or active LocalCI work.

```
localci cancel [flags]
```

### Examples

```
localci cancel
localci cancel --task noisy-fail
localci cancel --commit 'HEAD*' --task test
```

### Options

```
      --commit string   commit ref; defaults depend on command
  -h, --help            help for cancel
      --json            print JSON output
      --no-clone        run against or select the working tree instead of an isolated clone
      --repo string     repo directory; defaults to nearest Git repo ancestor
      --task string     full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

