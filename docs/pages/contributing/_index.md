---
title: "Contributing"
---

:+1::tada: First off, thank you for taking the time to contribute to Liqo! :tada::+1:

The following is a set of guidelines for contributing to Liqo, which are hosted in the liqotech Organization on GitHub. 
These are mostly guidelines, not rules. Use your best judgment, and feel free to propose changes to this document
in a pull request.

## Your First Code Contribution

Unsure where to begin contributing to Liqo? You can start by looking through the help-wanted issues.

### Repository Structure

The liqo repository structure follows the [Golang standard layout](https://github.com/golang-standards/project-layout). 
All components have the same structure:

If you want to read about using Liqo or developing packages in Liqo, the Liqo Flight Manual is free and available online. 

## Changelog Creation

Liqo leverages lerna-changelog to create the changelog of a certain version. PRs with the following labels applied will be considered for the changelog:

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