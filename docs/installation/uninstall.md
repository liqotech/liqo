# Uninstall

Liqo can be uninstalled by leveraging the dedicated *liqoctl* command:

```bash
liqoctl uninstall
```

Alternatively, the same operation can be performed directly with Helm:

```bash
helm uninstall liqo --namespace liqo
```

```{admonition} Note
Due to current limitations, the uninstallation process might hang in case peerings are still established, or namespaces are selected for offloading.
To this end, *liqoctl* performs a set of pre-checks and aborts the process in case any of the above is found, requesting the administrator to **unpeer all clusters and unoffload all namespaces** with the dedicated *liqoctl* commands.
```

## Purge CRDs

By default, the uninstallation process does not remove the Liqo CRDs and the system namespaces.
These operations can be performed by adding the `--purge` flag:

```bash
liqoctl uninstall --purge
```
