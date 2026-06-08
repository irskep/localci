## localci cat

Print one artifact to stdout

### Synopsis

Print one selected task artifact to stdout. With no artifact name, cat prints the primary artifact.

```
localci cat [artifact-name] [flags]
```

### Examples

```
localci cat --failed
localci cat --task noisy-fail
localci cat report.txt --task build
localci cat dist/app.tar.gz --task build --raw
```

### Options

```
      --attempt int     attempt number; defaults to latest attempt
      --commit string   commit ref; defaults depend on command
      --failed          show artifacts for failed and timed-out tasks
  -h, --help            help for cat
      --primary         show primary artifacts
      --raw             print raw artifact bytes
      --repo string     repo directory; defaults to nearest Git repo ancestor
      --task string     full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

