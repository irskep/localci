## localci completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

```sh
localci completion fish | source
```

To load completions for every new session, execute once:

```sh
localci completion fish > ~/.config/fish/completions/localci.fish
```

You will need to start a new shell for this setup to take effect.


```
localci completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### SEE ALSO

* [localci completion](../localci_completion/)	 - Generate the autocompletion script for the specified shell

