## localci run

Queue a daemon-managed run manually

### Synopsis

Queue a daemon-managed run manually. Use --no-clone to run against the live working tree.

```
localci run [flags]
```

### Examples

```
localci run --wait
localci run --no-clone --wait
localci run --no-clone --task test --wait
```

### Options

```
      --annotation stringArray   attach a run annotation as key=value
      --commit string            commit ref; defaults depend on command
  -h, --help                     help for run
      --json                     print JSON output
      --no-clone                 run against or select the working tree instead of an isolated clone
      --repo string              repo directory; defaults to nearest Git repo ancestor
      --task string              full task name or unambiguous short task name
      --wait                     wait for the selected run to complete
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

