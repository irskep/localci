## localci postcommit

Enqueue tasks for a committed revision

### Synopsis

Enqueue tasks for a committed revision. This is the command the Git post-commit hook calls.

```
localci postcommit [flags]
```

### Examples

```
localci postcommit --commit HEAD
localci postcommit --commit HEAD --task test
localci postcommit --repo "$repo" --commit "$commit"
```

### Options

```
      --annotation stringArray   attach a run annotation as key=value
      --commit string            commit ref; defaults depend on command
  -h, --help                     help for postcommit
      --json                     print JSON output
      --repo string              repo directory; defaults to nearest Git repo ancestor
      --task string              full task name or unambiguous short task name
```

### SEE ALSO

* [localci](../localci/)	 - Local post-commit validation runner

