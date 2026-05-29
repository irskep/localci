## localci dash

Open the terminal UI

### Synopsis

Open the terminal UI. It uses the daemon's REST and websocket APIs and defaults to the all-repos home view when no target is provided.

```
localci dash [flags]
```

### Examples

```
localci dash
localci dash --task noisy-fail
```

### Options

```
      --artifact string   artifact display name
      --attempt int       attempt number; defaults to latest attempt
      --commit string     commit ref; defaults depend on command
  -h, --help              help for dash
      --no-clone          run against or select the working tree instead of an isolated clone
      --repo string       repo directory; defaults to nearest Git repo ancestor
      --task string       full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

