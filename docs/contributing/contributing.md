# Contributing to Liqo

First off, thank you for taking the time to contribute to Liqo!

This page lists a set of contributing guidelines, including suggestions about the local development of Liqo components and the execution of the automatic tests.

## Repository structure

The Liqo repository structure follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).

## Commits convention

The project adopts the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification to classify commits, according to the nature of the change.
In summary, commit messages should follow the following structure:

```text
<type>[optional scope]: <description>`  

[optional body]

[optional footer(s)]
```

where `<type>` is one of:

* `feat`: 🚀 New Features
* `fix`: 🐛 Bug Fixes
* `refactor`: 🧹 Code Refactoring
* `docs`: 📝 Documentation
* `style`: 💄Code Style
* `perf`: 🐎 Performance Improvement
* `test`: ✅ Tests
* `chore`: 🚚 Dependencies Management
* `build`: 📦 Builds Management
* `ci`: 👷 CI/CD
* `revert`: ⏪ Reverts Previous Changes

## PRs convention

PRs are automatically assigned the label according to the `<type>` specified in the commit message(s) (see [commits convention section](#commits-convention)).
The PR's name can contain an optional scope as prefix (e.g., `[<scope>]: ...`) indicating which component(s) is being involved in the PR.

## Release notes generation

Liqo leverages the automatic release notes generation capabilities featured by GitHub.
Specifically, PRs characterized by the labels described in the [PRs convention section](#prs-convention) get included in the respective category.

## Local development

While developing a new feature, it is typically useful to test the changes in a local environment, as well as debug the code to identify possible problems.
To this end, you can leverage the *setup.sh* script provided for the *quick start example* to spawn two development clusters using [KinD](https://kind.sigs.k8s.io/), and then install Liqo on both of them (you can refer to the [dedicated section](InstallationDevelopmentVersions) for additional information concerning the installation of development versions through liqoctl):

```bash
./examples/quick-start/setup.sh
liqoctl install kind --kubeconfig=./liqo_kubeconf_rome --version ...
liqoctl install kind --kubeconfig=./liqo_kubeconf_milan --version ...
```

Once the environment is properly setup, it is possible to proceed according to one of the following approaches:

* Building and pushing the Docker image of the component under development to a registry, and appropriately editing the corresponding *Deployment/DaemonSet* to use the custom version.
  This allows to observe the modified component in realistic conditions, and it is mandatory for the networking substratum, since it needs to interact with the underlying host configuration.
* Scaling to 0 the number of replicas of the component under development, copying its current configuration (i.e., command-line flags), and executing it locally (while targeting the appropriate cluster).
  This allows for faster development cycles, as well as for the usage of standard debugging techniques to troubleshoot possible issues.

## Automatic tests

Liqo features two major test suites:

* *End-to-end (E2E) tests*, which assess the correct functioning of the main Liqo features.
* *Unit Tests*, which focus on each specific component, in multiple operating conditions.

Both test suites are automatically executed through the GitHub Actions pipelines, following the corresponding slash command trigger.
A successful outcome is required to make PRs eligible for being considered for review and merged.

The following sections provide additional details concerning how to run the above tests in a local environment, for troubleshooting.

### End-to-end tests

We suggest executing the E2E tests on a system with at least 8 GB of free RAM.
Additionally, please review the requirements presented in the [Liqo examples section](/examples/requirements.md), which also apply in this case (including the suggestions concerning increasing the maximum number of *inotify* watches).

Once all requirements are met, it is necessary to export the set of environment variables shown below, to configure the tests.
In most scenarios, the only variable that needs to be modified is `LIQO_VERSION`, which should point to the SHA of the commit referring to the Liqo development version to be tested (the appropriate Docker images shall have been built in advance through the appropriate GitHub Actions pipeline).

```bash
export CLUSTER_NUMBER=2
export K8S_VERSION=v1.29.2
export INFRA=kind
export CNI=kindnet
export TMPDIR=$(mktemp -d)
export BINDIR=${TMPDIR}/bin
export KUBECTL=${BINDIR}/kubectl
export LIQOCTL=${BINDIR}/liqoctl
export HELM=${BINDIR}/helm
export KUBECONFIGDIR=${TMPDIR}/kubeconfigs
export TEMPLATE_DIR=${PWD}/test/e2e/pipeline/infra/kind
export NAMESPACE=liqo
export LIQO_VERSION=<YOUR_COMMIT_ID>
export POD_CIDR_OVERLAPPING=false
export TEMPLATE_FILE=cluster-templates.yaml.tmpl
```

Finally, it is possible to launch the tests:

```bash
make e2e
```

### Unit tests

Most unit tests can be run directly using [the *ginkgo* CLI](https://onsi.github.io/ginkgo/#installing-ginkgo), which in turn supports the standard testing API (*go test*, IDE features, ...).
The only requirement is the [controller-runtime envtest environment](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest), which can be installed through [`setup-envtest`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/tools/setup-envtest):

```bash
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
setup-envtest use 1.25.x!
```

To enable the downloaded envtest, you can append the following line to your `~/.bashrc` or `~/.zshrc` file:

```bash
source <(setup-envtest use --installed-only --print env 1.25.x)
```

Some networking tests, however, require an isolated environment.
To this end, you can leverage the dedicated *liqo-test* Docker image (the Dockerfile is available in *build/liqo-test*):

```bash
# Build the liqo-test Docker image
make test-container

# Run all unit tests
make unit

# Run the tests for a specific package.
# Note, the package path must start with ./ to avoid the "package ... is not in GOROOT error".
make unit PACKAGE_PATH=<package_path>
```

#### Debugging unit tests

When executing the unit tests from the *liqo-test* container, it is possible to use Delve to perform remote debugging:

1. Start the *liqo-test* container with an idle entry point, exposing a port of choice (e.g. 2345):

   ```bash
   docker run --name=liqo-test -d -p 2345:2345 --mount type=bind,src=$(pwd),dst=/go/src/liqo \
      --privileged=true --workdir /go/src/liqo --entrypoint="" liqo-test tail -f /dev/null
   ```

2. Open a shell inside the *liqo-test* container, and install Delve:

   ```bash
   docker exec -it liqo-test bash
   go install github.com/go-delve/delve/cmd/dlv@latest
   ```

3. Run a specific test inside the container:

   ```bash
   dlv test --headless --listen=:2345 --api-version=2  \
      --accept-multiclient ./path/to/test/directory
   ```

4. From the host, connect to *localhost:2345* with your remote debugging client of choice, and enjoy!
