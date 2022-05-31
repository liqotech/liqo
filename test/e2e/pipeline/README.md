# Pipeline Steps

## Steps

* **Infrastructure Provisioning** (`infra` directory): the infrastructure provisioner should implement the following "interface":
  * `pre-requirements.sh`: install the dependency tools for the cluster provisioning.
  * `setup.sh`: provision the testing environments. It stores the KUBECONFIGS to access the clusters in a directory referenced in `${TMPDIR}/kubeconfigs/`.
  The amount of clusters to instantiate is available in `CLUSTER_NUMBER` variable.
  * `clean-up.sh`: destroy the testing environment
* **Liqo Installation** (`installer` directory): installs Liqo on the clusters
  * `install.sh`: install Liqo.
  * `uninstall.sh`: uninstall Liqo.
* **Testing**:
  * *Post-Install*: test Liqo just completed installation.
  They should be used to assert that Liqo is correctly installed.
  * *Cruise*: tests that validate user-oriented Liqo features.
  Test suites are executed without a precise order, so they should be self-contained.
  * *Uninstall*: tests executed after the uninstall of Liqo.

In addition, an extra hook is always executed even in presence of failures during the pipeline execution:

* **Diagnostic** ( `diagnostic/diagnose.sh` entrypoint): take a snapshot of the situation at the end of the pipeline. It can be used to retrieve information about resources on the clusters or the systems where the tests are executed.

## Input Variables

* *CLUSTER_NUMBER*: the number of clusters part of the infrastructure
* *K8S_VERSION*: the target version of Kubernetes
* *CNI*: the CNI to install in testing clusters
* *TMPDIR*: the temporary directory where the workflow is executed
* *BINDIR*: the temporary directory where the required binaries are stored
