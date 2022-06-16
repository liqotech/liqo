# Liqo CLI tool

## Introduction

Liqoctl is the CLI tool to streamline the **installation** and **management** of Liqo.
Specifically, it abstracts the creation and modification of Liqo defined custom resources, allowing to:

* **Install/uninstall** Liqo, wrapping the corresponding Helm commands and automatically retrieving the appropriate parameters based on the target cluster configuration.
* Establish and revoke **peering** relationships towards remote clusters.
* Enable and configure **workload offloading** on a per-namespace basis.
* Retrieve the **status** of Liqo, as well as of given peering relationships and offloading setups.

```{admonition} Note
*liqoctl* displays a *kubectl* compatible behavior concerning Kubernetes API access, hence supporting the `KUBECONFIG` environment variable, as well as the standard flags, including `--kubeconfig` and `--context`.
```

## Install liqoctl with Homebrew

If you are using the [Homebrew](https://brew.sh/) package manager, you can install *liqoctl* with Homebrew:

```bash
brew install liqoctl
```

When installed with Homebrew, autocompletion scripts are automatically configured and should work out of the box.

## Install liqoctl manually

You can download and install *liqoctl* manually, following the appropriate instructions based on your operating system and architecture.

`````{tab-set}
````{tab-item} Linux

Download *liqoctl* and move it a to a file location in your system `PATH`:

**AMD64:**

```bash
curl --fail -LS --output liqoctl "https://get.liqo.io/liqoctl-linux-amd64"
sudo install -o root -g root -m 0755 liqoctl /usr/local/bin/liqoctl
```

**ARM64:**

```bash
curl --fail -LS --output liqoctl "https://get.liqo.io/liqoctl-linux-arm64"
sudo install -o root -g root -m 0755 liqoctl /usr/local/bin/liqoctl
```

```{admonition} Note
Make sure `/usr/local/bin` is in your `PATH` environment variable.
```
````

````{tab-item} MacOS

Download *liqoctl*, make it executable, and move it a to a file location in your system `PATH`:

**Intel:**

```bash
curl --fail -LS --output liqoctl "https://get.liqo.io/liqoctl-darwin-amd64"
chmod +x liqoctl
sudo mv liqoctl /usr/local/bin/liqoctl
```

**Apple Silicon:**

```bash
curl --fail -LS --output liqoctl "https://get.liqo.io/liqoctl-darwin-arm64"
chmod +x liqoctl
sudo mv liqoctl /usr/local/bin/liqoctl
```

```{admonition} Note
Make sure `/usr/local/bin` is in your `PATH` environment variable.
```
````

````{tab-item} Windows

Download the *liqoctl* binary:

```bash
curl --fail -LSO "https://get.liqo.io/liqoctl-windows-amd64"
```

And move it to a file location in your system `PATH`.

````
`````

Alternatively, you can download *liqoctl* from the [Liqo releases](https://github.com/liqotech/liqo/releases/) page on GitHub.

### Enable shell autocompletion

*liqoctl* provides autocompletion support for Bash, Zsh, Fish, and PowerShell.

`````{tab-set}
````{tab-item} Bash

The *liqoctl* completion script for Bash can be generated with the `liqoctl completion bash` command.
Sourcing the completion script in your shell enables *liqoctl* autocompletion.

However, the completion script depends on *bash-completion*, which means that you have to install this software first.
You can test if you have *bash-completion* already installed by running `type _init_completion`.
If it returns an error, you can install it via your OS's package manager.

To load completions in your current shell session:

```bash
source <(liqoctl completion bash)
```

To load completions for every new session, execute once:

```bash
source <(liqoctl completion bash) >> ~/.bashrc
```

After reloading your shell, *liqoctl* autocompletion should be working.
````

````{tab-item} Zsh

The *liqoctl* completion script for Zsh can be generated with the `liqoctl completion zsh` command.

If shell completion is not already enabled in your environment, you will need to enable it.
You can execute the following once:

```zsh
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions for each session, execute once:

```zsh
liqoctl completion zsh > "${fpath[1]}/_liqoctl"
```

After reloading your shell, *liqoctl* autocompletion should be working.
````

````{tab-item} Fish

The *liqoctl* completion script for Fish can be generated with the `liqoctl completion fish` command.

To load completions in your current shell session:

```fish
liqoctl completion fish | source
```

To load completions for every new session, execute once:

```fish
liqoctl completion fish > ~/.config/fish/completions/liqoctl.fish
```

After reloading your shell, *liqoctl* autocompletion should be working.
````

````{tab-item} PowerShell

The *liqoctl* completion script for PowerShell can be generated with the `liqoctl completion powershell` command.

To load completions in your current shell session:

```powershell
liqoctl completion powershell | Out-String | Invoke-Expression
```

To load completions for every new session, add the output of the above command
to your PowerShell profile.

After reloading your shell, *liqoctl* autocompletion should be working.
````


`````
