# Contributing to Liqo

First off, thank you for taking the time to contribute to Liqo!

This page lists a set of contributing guidelines, including suggestions about the local development of Liqo components and the execution of the automatic tests.

## Repository structure

The Liqo repository structure follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).

## Release notes generation

Liqo leverages the automatic release notes generation capabilities featured by GitHub.
Specifically, PRs characterized by the following labels get included in the respective category:

* *kind/breaking*: 💥 Breaking Change
* *kind/feature*: 🚀 New Features
* *kind/bug*: 🐛 Bug Fixes
* *kind/cleanup*: 🧹 Code Refactoring
* *kind/docs*: 📝 Documentation

## Local development

While developing a new feature, it is typically useful to test the changes in a local environment, as well as debug the code to identify possible problems.
To this end, you can leverage the *setup.sh* script provided for the *quick start example* to spawn two development clusters using [Kind](https://kind.sigs.k8s.io/), and then install Liqo on both of them (you can refer to the [dedicated section](InstallationDevelopmentVersions) for additional information concerning the installation of development versions through liqoctl):

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

Both test suites are automatically executed through the GitHub action pipelines, following the corresponding slash command trigger.
A successful outcome is required to make PRs eligible for being considered for review and merged.

The following sections provide additional details concerning how to run the above tests in a local environment, for troubleshooting.

### End-to-end tests

We suggest executing the E2E tests on a system with at least 8 GB of free RAM.
Additionally, please review the requirements presented in the [Liqo examples section](/examples/requirements.md), which also apply in this case (including the suggestions concerning increasing the maximum number of *inotify* watches).

Once all requirements are met, it is necessary to export the set of environment variables shown below, to configure the tests.
In most scenarios, the only variable that needs to be modified is `LIQO_VERSION`, which should point to the SHA of the commit referring to the Liqo development version to be tested (the appropriate Docker images shall have been built in advance through the appropriate GitHub actions pipeline).

```bash
export CLUSTER_NUMBER=4
export K8S_VERSION=v1.21.1
export CNI=kindnet
export TMPDIR=$(mktemp -d)
export BINDIR=${TMPDIR}/bin
export TEMPLATE_DIR=${PWD}/test/e2e/pipeline/infra/kind
export NAMESPACE=liqo
export KUBECONFIGDIR=${TMPDIR}/kubeconfigs
export LIQO_VERSION=<YOUR_COMMIT_ID>
export INFRA=kind
export LIQOCTL=${BINDIR}/liqoctl
export POD_CIDR_OVERLAPPING=false
export TEMPLATE_FILE=cluster-templates.yaml.tmpl
```

Finally, it is possible to launch the tests:

```bash
make e2e
```

### Unit tests

Most unit tests can be run directly using [the *ginkgo* CLI](https://onsi.github.io/ginkgo/#installing-ginkgo), which in turn supports the standard testing API (*go test*, IDE features, ...).
The only requirement is the [controller-runtime envtest environment](https://book.kubebuilder.io/reference/envtest.html), which can be installed through:

```bash
export K8S_VERSION=1.19.2
curl --fail -sSLo envtest-bins.tar.gz "https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-${K8S_VERSION}-$(go env GOOS)-$(go env GOARCH).tar.gz"
mkdir /usr/local/kubebuilder/
tar -C /usr/local/kubebuilder/ --strip-components=1 -zvxf envtest-bins.tar.gz
```

Some networking tests, however, require an isolated environment.
To this end, you can leverage the dedicated *liqo-test* Docker image (the Dockerfile is available in *build/liqo-test*):

```bash
# Run all unit tests
docker run --rm -v $PATH_TO_LIQO:/go/src/github.com/liqotech/liqo liqo-test

# Run the tests for a specific package
docker run --rm --entrypoint="" -v $PATH_TO_LIQO:/go/src/github.com/liqotech/liqo liqo-test go test $PACKAGE
```

#### Debugging unit tests

When executing the unit tests from the *liqo-test* container, it is possible to use Delve to perform remote debugging:

1. Start the *liqo-test* container with an idle entry point, exposing a port of choice (e.g. 2345):

   ```bash
   docker run --name=liqo-test -d -p 2345:2345 --entrypoint="" \
      -v $PATH_TO_LIQO:/go/src/github.com/liqotech/liqo liqo-test tail -f /dev/null

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

4. From the host, connect to *localhost:2345* with your remote debugging client of choice (e.g. [GoLand](https://www.jetbrains.com/help/go/attach-to-running-go-processes-with-debugger.html#step-3-create-the-remote-run-debug-configuration-on-the-client-computer)), and enjoy!
