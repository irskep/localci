## localci restart

Restart the daemon

### Synopsis

Finish current work, then restart the daemon. Queued tasks resume after restart. Use --immediate to interrupt current work and retry it after restart.

```
localci restart [flags]
```

### Options

```
  -h, --help        help for restart
      --immediate   Interrupt current work instead of waiting for it to finish
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

