---
title: "Contributing"
---

:+1::tada: First off, thank you for taking the time to contribute to Liqo! :tada::+1:

The following is a set of guidelines for contributing to Liqo, which are hosted in the liqotech Organization on GitHub. 

## Your First Code Contribution

Unsure where to begin contributing to Liqo? You can start by looking through the help-wanted issues.

### Repository Structure

The liqo repository structure follows the [Golang standard layout](https://github.com/golang-standards/project-layout). 

## Changelog Creation

Liqo leverages `heinrichreimer/github-changelog-generator-action` to create the changelog of a certain version. 
PRs with the following labels applied will be considered for the changelog:

* breaking (:boom: Breaking Change)
* enhancement (:rocket: Enhancement)
* bug (:bug: Bug Fix)
* documentation (:memo: Documentation)

## Local development

Liqo components can be developed locally. We provide a [deployment script](/examples/kind.sh) to spawn multiple 
kubernetes clusters by using [Kind](https://kind.sigs.k8s.io/) with Liqo installed. This script can be used as a starting
point to improve/replace one Liqo component:

  1. deploy Liqo on a local Kind cluster;
  2. `describe` the pod running the component you wish to replace, and copy its command-line flags;
  3. `scale --replicas=0` the component;
  4. run the component on the host, with the command-line flags you copied before.

### Testing

Liqo relies on two major test suites:

* **E2E tests**, which assess the functioning of Liqo features from user perspective,
* **Unit Tests**.
 
In this section, you can find how you can execute on your systems those test suites.

#### End-to-End (E2E)

To run on your local environment the E2E tests, you should:

0. We suggest to run the E2E tests on a system with at least 8 GB of free main memory.
1. Configure your system. In particular, the E2E tests require four clusters to be spawned.
   To do so, you should increase the `fs.inotify.max_user_watches` and `fs.inotify.max_user_instances` to correctly create the 4 clusters:

```bash
sudo sysctl fs.inotify.max_user_watches=52428899
sudo sysctl fs.inotify.max_user_instances=2048
```

Alternatively, you can persist them through the `sysctl.conf` file, in order to have them always set:

```bash
echo fs.inotify.max_user_watches=52428899 | sudo tee -a /etc/sysctl.conf  
echo fs.inotify.max_user_instances=2048 | sudo tee -a /etc/sysctl.conf  
sudo sysctl -p
```

2. Export the required variables to run the E2E tests. 
They are also listed in the Makefile before the e2e target.

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
export DOCKER_USERNAME=<your-username>
export DOCKER_PASSWORD=<your-password>
```

3. Run the e2e tests
```
make e2e
```

#### Unit Tests

Most tests can be run "directly" using `ginkgo`, which in turn supports the standard testing API (`go test`, IDE features, ...). Some tests however require an isolated environment, for which you can use the `liqo-test` Docker image (in `build/liqo-test`). Just mount the repository in `/go/src/github.com/liqotech/liqo` inside the container:

```sh
docker run --name=liqo-test -v $PATH_TO_LIQO:/go/src/github.com/liqotech/liqo liqo-test

# To run a specific test
docker run --name=liqo-test -v $PATH_TO_LIQO:/go/src/github.com/liqotech/liqo liqo-test --entrypoint="" go test $PACKAGE
```

#### Debugging tests

If you want to debug tests, you can use Delve for remote debugging:

  1. Start the container with an idle entrypoint, exposing a port of choice (e.g. 2345):

```sh
docker run --name=liqo-test -d -p 2345:2345 -v $PATH_TO_LIQO:/go/src/github.com/liqotech/liqo --entrypoint="" liqo-test tail -f /dev/null
```

  2. Open a shell into the container:

```sh
docker exec -it liqo-test bash
```

  3. Once inside the container, install Delve:

```sh
go install github.com/go-delve/delve/cmd/dlv@latest
```

  4. Run a specific test inside the container: (note that `$TEST_PATH` must refer to a directory)

```sh
dlv test --headless --listen=:2345 --api-version=2 --accept-multiclient ./$TEST_PATH
```

  5. From the host, connect to `localhost:2345` with your remote debugging client of choice (e.g. [GoLand](https://www.jetbrains.com/help/go/attach-to-running-go-processes-with-debugger.html#step-3-create-the-remote-run-debug-configuration-on-the-client-computer)).

## Pull Requests

The process described here has several goals:

* Maintain and possibly improve Liqo's quality
* Fix problems that are important to users
* Engage the community in working toward the best possible Liqo and to embrace new possible use-cases

## Styleguides

### Git Commit Messages

* Use the present tense ("Add feature" not "Added feature")
* Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
* Limit the first line to 72 characters or less
* Reference issues and pull requests liberally after the first line

## Credits

[Atom Contributing Guidelines](https://github.com/atom/atom/blob/master/CONTRIBUTING.md) inspired us when writing this 
document. Many thanks!
