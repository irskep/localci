## localci completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

```sh
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions in your current shell session:

```sh
source <(localci completion zsh)
```

To load completions for every new session, execute once:

#### Linux:

```sh
localci completion zsh > "${fpath[1]}/_localci"
```

#### macOS:

```sh
localci completion zsh > $(brew --prefix)/share/zsh/site-functions/_localci
```

You will need to start a new shell for this setup to take effect.


```
localci completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### SEE ALSO

* [localci completion](../localci_completion/)	 - Generate the autocompletion script for the specified shell

