# Stateful Applications

This tutorial demonstrates how to use the core Liqo features to deploy stateful applications.
In particular, you will deploy a multi-master *mariadb-galera* database across a multi-cluster environment (composed of two clusters, respectively identified as *Turin* and *Lyon*), hence replicating the data in multiple regions.

## Provision the playground

First, check that you are compliant with the [requirements](/examples/requirements.md).

Then, let's open a terminal on your machine and launch the following script, which creates the two above-mentioned clusters with KinD and installs Liqo on them.
Each cluster is made by a single combined control-plane + worker node.

{{ env.config.html_context.generate_clone_example('stateful-applications') }}

Export the kubeconfigs environment variables to use them in the rest of the tutorial:

```{warning}
The install script creates two clusters with no overlapping pod CIDRs.
This is required by the *mariadb-galera* application to work correctly.
Given it needs to know the real IP of the connected masters, it will not work correctly when natting is enabled.
```

```bash
export KUBECONFIG="$PWD/liqo_kubeconf_turin"
export KUBECONFIG_LYON="$PWD/liqo_kubeconf_lyon"
```

```{admonition} Note
We suggest exporting the kubeconfig of the first cluster as default (i.e., `KUBECONFIG`), since it will be the entry point of the virtual cluster and you will mainly interact with it.
```

## Peer the clusters

Once Liqo is installed in your clusters, you can establish new *peerings*:

```bash
liqoctl peer --remote-kubeconfig "$KUBECONFIG_LYON" --gw-server-service-type NodePort
```

When the above command returns successfully, you can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that an outgoing peering is currently active towards the *Lyon* cluster:

```text
NAME   ROLE       AGE
lyon   Provider   18s
```

## Deploy a stateful application

In this step, you will deploy a *mariadb-galera* database using the [Bitnami helm chart](https://bitnami.com/stack/mariadb-galera/helm).

First, you need to add the helm repository:

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
```

```{admonition} Tip
You can install the Helm package manager by checking its [**documentation**](https://helm.sh/docs/intro/install/).
```

Then, create the namespace and offload it to remote clusters:

```bash
kubectl create namespace liqo-demo
liqoctl offload namespace liqo-demo --namespace-mapping-strategy EnforceSameName
```

This command will create a twin `liqo-demo` namespace in the *Lyon* cluster.
Refer to the dedicated [usage page](/usage/namespace-offloading.md) for additional information concerning namespace offloading configurations.

Now, deploy the helm chart:

```bash
helm install db bitnami/mariadb-galera -n liqo-demo -f manifests/values.yaml
```

The release is configured to:

* have two replicas;
* spread the replicas across the cluster (i.e., a hard pod anti-affinity is set);
* use the [`liqo` virtual storage class](UsageStatefulApplicationsVirtualStorageClass).

Check that these constraints are met by typing:

```bash
kubectl get pods -n liqo-demo -o wide
```

After a while (the startup process might require a few minutes), you should see two replicas of a *StatefulSet* spread over two different nodes (one local and one remote).

```text
NAME                  READY   STATUS    RESTARTS   AGE     IP            NODE                  NOMINATED NODE   READINESS GATES
db-mariadb-galera-0   1/1     Running   0          3m13s   10.210.0.15   liqo-lyon             <none>           <none>
db-mariadb-galera-1   1/1     Running   0          2m6s    10.200.0.17   turin-control-plane   <none>           <none>
```

## Consume the database

When the database is up and running, check that it is operating as expected executing a simple SQL client in your cluster:

```bash
kubectl run db-mariadb-galera-client --rm --tty -i \
    --restart='Never' --namespace default \
    --image docker.io/bitnami/mariadb-galera:10.6.7-debian-10-r56 \
    --command \
    -- mysql -h db-mariadb-galera.liqo-demo -uuser -ppassword my_database
```

And then create an example table and insert some data:

```sql
CREATE TABLE People (
    PersonID int,
    LastName varchar(255),
    FirstName varchar(255),
    Address varchar(255),
    City varchar(255)
);

INSERT INTO People
    (PersonID, LastName, FirstName, Address, City)
    VALUES
    (1, 'Smith', 'John', '123 Main St', 'Anytown');
```

You are now able to query the database and grab the data:

```sql
SELECT * FROM People;
```

```text
+----------+----------+-----------+-------------+---------+
| PersonID | LastName | FirstName | Address     | City    |
+----------+----------+-----------+-------------+---------+
|        1 | Smith    | John      | 123 Main St | Anytown |
+----------+----------+-----------+-------------+---------+
1 row in set (0.000 sec)
```

### Database failures toleration

With this setup the applications running on a cluster can tolerate failures of the local database replica.

This can be checked deleting one of the replicas:

```bash
kubectl delete pod db-mariadb-galera-0 -n liqo-demo
```

And querying again for your data:

```bash
kubectl run db-mariadb-galera-client --rm --tty -i \
    --restart='Never' --namespace default \
    --image docker.io/bitnami/mariadb-galera:10.6.7-debian-10-r56 \
    --command \
      -- mysql -h db-mariadb-galera.liqo-demo -uuser -ppassword my_database \
      --execute "SELECT * FROM People;"
```

```{admonition} Pro-tip
Try deleting the other replica and query again.

**NOTE**: at least one of the two replicas should be always running, be careful deleting all of them.
```

```{admonition} Note
You can run exactly the same commands to query the data from the other cluster, and you will get the same results.
```

## Tear down the playground

Our example is finished; now we can remove all the created resources and tear down the playground.

### Unoffload namespaces

Before starting the uninstallation process, make sure that all namespaces are unoffloaded:

```bash
liqoctl unoffload namespace liqo-demo
```

Every pod that was offloaded to a remote cluster is going to be rescheduled onto the local cluster.

### Revoke peerings

Similarly, make sure that all the peerings are revoked:

```bash
liqoctl unpeer --remote-kubeconfig "$KUBECONFIG_LYON" --skip-confirm
```

At the end of the process, the virtual node is removed from the local cluster.

### Uninstall Liqo

Now you can remove Liqo from your clusters with *liqoctl*:

```bash
liqoctl uninstall --skip-confirm
liqoctl uninstall --kubeconfig="$KUBECONFIG_LYON" --skip-confirm
```

```{admonition} Purge
By default the Liqo CRDs will remain in the cluster, but they can be removed with the `--purge` flag:

```bash
liqoctl uninstall --purge
liqoctl uninstall --kubeconfig="$KUBECONFIG_LYON" --purge
```

### Destroy clusters

To teardown the KinD clusters, you can issue:

```bash
kind delete cluster --name turin
kind delete cluster --name lyon
```
