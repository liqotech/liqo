# Liqo CLI tool

## Introduction

Liqoctl is the CLI tool to streamline the **installation** and **management** of Liqo.

Specifically, it abstracts the creation and modification of Liqo-defined custom resources, allowing to:

* **Install/uninstall** Liqo, wrapping the corresponding **Helm** commands and automatically retrieving the appropriate parameters based on the target cluster configuration.
* Establish and revoke **peering** relationships towards remote clusters.
* Enable and configure **workload offloading** on a per-namespace basis.
* Retrieve the **status** of Liqo, as well as of given peering relationships and offloading setups.

Liqoctl uses the REST interface to interact with the **Kubernetes API server**.
Hence, it can be executed on any host that has connectivity towards the Kubernetes API server.
In other words, liqoctl does not need to be installed on the cluster nodes, but only on the machine used to manage Liqo (like you do with `kubectl` or `helm`), which cannot even belong to any Kubernetes cluster.

![Peering architecture](/_static/images/installation/liqoctl/liqoctl.drawio.svg)

```{warning}
Make sure to always **use the *liqoctl* version matching the one of Liqo** installed (or to be installed) in your cluster(s).
```

{{ env.config.html_context.generate_liqoctl_version_warning() }}

```{admonition} Note
*liqoctl* displays a *kubectl* compatible behavior concerning Kubernetes API access, hence supporting the `KUBECONFIG` environment variable, as well as all the standard flags, including `--kubeconfig` and `--context`.
Moreover, subcommands interacting with two clusters (e.g., *liqoctl peer*) feature a parallel set of flags concerning Kubernetes API access to the remote cluster, in the form `--remote-<flag>` (e.g., `--remote-kubeconfig`, `--remote-context`).
```

(InstallationLiqoctlWithHomebrew)=

## Install liqoctl with Homebrew

If you are using the [Homebrew](https://brew.sh/) package manager, you can install *liqoctl* with Homebrew:

```bash
brew install liqoctl
```

When installed with Homebrew, autocompletion scripts are automatically configured and should work out of the box.

(InstallationLiqoctlWithasdf)=

## Install liqoctl with asdf

If you are using the [asdf](https://asdf-vm.com/) runtime manager, you can install *liqoctl* with asdf:

```bash
# Add the liqoctl plugin for asdf
asdf plugin add liqoctl

# List all installable versions
asdf list-all liqoctl

# Install the desired version
asdf install liqoctl <version>

# set it as the global version, unless a project declares it otherwise locally
asdf global liqoctl <version>
```

(InstallationLiqoctlManually)=

## Install liqoctl manually

You can download and install *liqoctl* manually, following the appropriate instructions based on your operating system and architecture.

`````{tab-set}
````{tab-item} Linux

Download *liqoctl* and move it to a file location in your system `PATH`:

**AMD64:**

{{ env.config.html_context.generate_liqoctl_install('linux', 'amd64') }}

**ARM64:**

{{ env.config.html_context.generate_liqoctl_install('linux', 'arm64') }}

```{admonition} Note
Make sure `/usr/local/bin` is in your `PATH` environment variable.
```
````

````{tab-item} MacOS

Download *liqoctl*, make it executable, and move it to a file location in your system `PATH`:

**Intel:**

{{ env.config.html_context.generate_liqoctl_install('darwin', 'amd64') }}

**Apple Silicon:**

{{ env.config.html_context.generate_liqoctl_install('darwin', 'arm64') }}

```{admonition} Note
Make sure `/usr/local/bin` is in your `PATH` environment variable.
```
````

````{tab-item} Windows

Download the *liqoctl* binary:

{{ env.config.html_context.generate_liqoctl_install('windows', 'amd64') }}

And move it to a file location in your system `PATH`.

````
`````

Alternatively, you can manually download *liqoctl* from the [Liqo releases](https://github.com/liqotech/liqo/releases/) page on GitHub.

## Install Kubectl plugin with Krew

You can install liqoctl as a kubectl plugin by using the [Krew](https://krew.sigs.k8s.io/) plugin manager:

```bash
kubectl krew install liqo
```

Then, all commands shall be invoked with `kubectl liqo` rather than `liqoctl`, although all functionalities remain the same.

```{warning}
While the kubectl plugin is supported, it is recommended to use liqoctl as this enables a better experience via tab auto-completion.
[Install it with Homebrew](InstallationLiqoctlWithHomebrew) if available on your system or by [manually downloading the binary from GitHub](InstallationLiqoctlManually).
```

(InstallationLiqoctlFromSource)=

## Install liqoctl from source

You can install *liqoctl* building it from source.
To do so, clone the Liqo repository, build the *liqoctl* binary, and move it to a file location in your system `PATH`:

```bash
git clone https://github.com/liqotech/liqo.git
cd liqo
make ctl
mv liqoctl /usr/local/bin/liqoctl
```

## Enable shell autocompletion

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
