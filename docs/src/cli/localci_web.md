## localci web

Open the web UI

### Synopsis

Open the web UI. It shows comprehensive information including logs, live status, downloadable binaries, and HTML task output.

```
localci web [flags]
```

### Examples

```
localci web
localci web --task noisy-fail
localci web --task noisy-fail --artifact combined.log
```

### Options

```
      --artifact string   artifact display name
      --attempt int       attempt number; defaults to latest attempt
      --commit string     commit ref; defaults depend on command
  -h, --help              help for web
      --no-clone          run against or select the working tree instead of an isolated clone
      --repo string       repo directory; defaults to nearest Git repo ancestor
      --task string       full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

