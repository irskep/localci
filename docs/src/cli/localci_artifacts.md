## localci artifacts

Print filesystem paths for task artifacts

### Synopsis

Print filesystem paths for task artifacts.

```
localci artifacts [flags]
```

### Examples

```
localci artifacts
localci artifacts --task noisy-fail
localci artifacts --failed --primary
localci artifacts --task noisy-fail --primary --paths-only
```

### Options

```
      --attempt int     attempt number; defaults to latest attempt
      --commit string   commit ref; defaults depend on command
      --failed          show artifacts for failed and timed-out tasks
  -h, --help            help for artifacts
      --json            print JSON output
      --paths-only      print only artifact paths
      --primary         show only primary artifacts
      --repo string     repo directory; defaults to nearest Git repo ancestor
      --task string     full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

