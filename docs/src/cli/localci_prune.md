## localci prune

Remove old or large task outputs

### Synopsis

Remove logs and artifacts across all repositories. By default, remove outputs older than 30 days, then the oldest remaining outputs until at most 10 GiB remain. Set either limit to 0 to disable it. Sizes accept decimal KB/MB/GB or binary KiB/MiB/GiB. Alternatively, use --larger-than to remove individual files above a size regardless of age. Active and queued runs, task records, and caches are preserved. Nothing is removed until you confirm or pass --yes.

```
localci prune [flags]
```

### Examples

```
localci prune --dry-run
localci prune
localci prune --max-age 7d --max-size 5GiB --yes
localci prune --larger-than 100MiB
```

### Options

```
      --dry-run              Preview removals without deleting anything
  -h, --help                 help for prune
      --larger-than string   Instead remove individual artifact files above this size (default "0")
      --max-age string       Remove run outputs older than this age (0 disables) (default "30d")
      --max-size string      Maximum combined output size; prune oldest runs first (0 disables) (default "10GiB")
      --yes                  Remove outputs without asking for confirmation
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

