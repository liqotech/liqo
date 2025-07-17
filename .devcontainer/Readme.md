# DevContainer

A devcontainer (short for "development container", https://containers.dev/) is a pre-configured, isolated environment defined by configuration files (like devcontainer.json and Dockerfiles) that sets up all the tools, dependencies, and settings needed for a project.
It allows developers to work in a consistent environment, regardless of their local setup, often using Visual Studio Code's "Remote - Containers" feature (https://code.visualstudio.com/docs/devcontainers/containers).
This helps avoid "it works on my machine" issues and simplifies onboarding for new contributors.

## Main Features

- **Go Development**: Includes Go and common Go utilities pre-installed and available on the `PATH`, along with the Go language extension for enhanced Go development.
- **Up-to-date Git**: Ships with an up-to-date version of Git, built from source if needed, and available on the `PATH`.
- **Docker CLI**: The Docker CLI (`docker`) and Docker Compose v2 are pre-installed, allowing container management using the **host's Docker daemon**.
- **GitHub CLI**: The latest GitHub CLI (`gh`) is installed and available on the `PATH`.  
- **Kubernetes in Docker (KinD)**: Includes KinD for running local Kubernetes clusters, supporting Kubernetes-native development and testing.
- **Enhanced Shell Experience**: Zsh is set as the default shell, with Oh My Zsh, Powerlevel10k theme, and a curated set of plugins (e.g., autosuggestions, syntax highlighting, kubectl, docker, git, golang, helm).
- **Host Home Directory Mount**: Mounts the host's home directory as read-only at `/mnt/hosthome` for easy access to configuration files and credentials.
- **Automatic Powerlevel10k Setup**: If a `.p10k.zsh` config is found in the host's home, it is copied and sourced automatically in the container.
- **Automatic Package Upgrades**: System packages are upgraded on container creation, including support for non-free packages.
- **Network Capabilities**: This devcontainer setup allows to carry out networking actions, such as creating Linux network namespaces and new network interfaces.

## Behaviours

- On container start, the Powerlevel10k theme is installed and configured if a `.p10k.zsh` file is present.
- Zsh plugins and Oh My Zsh plugins are installed for the `vscode` user.
