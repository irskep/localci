# Installation

LocalCI requires mise. Install mise first and make sure `mise` is available on `PATH`.

## Install LocalCI with mise

LocalCI should be installed as a mise-managed GitHub release tool. In the project or global mise config that should provide `localci`, add a GitHub backend entry for the repository that publishes the LocalCI binary:

```toml
[tools]
"github:irskep/localci" = "VERSION"
```

Use an exact released version. Do not rely on an unpinned floating version for project automation.

If the release archive needs disambiguation, use mise's GitHub backend options in that same tool entry, for example `asset_pattern` or executable naming options.

## Development checkout

When working inside this repository, use the repo's mise tasks:

```sh
mise install
mise run check
mise run daemon-start
```

The development checkout builds and runs LocalCI from source. That is for LocalCI development, not the recommended installation shape for projects that consume LocalCI.
