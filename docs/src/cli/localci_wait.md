## localci wait

Wait for a selected run or task to complete

### Synopsis

Wait for the selected run or task to complete and show the result.

```
localci wait [flags]
```

### Examples

```
localci wait
localci wait --task noisy-fail
localci wait --commit 'HEAD*'
```

### Options

```
      --commit string   commit ref; defaults depend on command
  -h, --help            help for wait
      --json            print JSON output
      --no-clone        run against or select the working tree instead of an isolated clone
      --repo string     repo directory; defaults to nearest Git repo ancestor
      --task string     full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

