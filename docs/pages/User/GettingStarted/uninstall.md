---
title: Uninstall Liqo
weight: 4
---

## Uninstall steps

This procedure uninstalls Liqo from your cluster.

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash -s -- --uninstall
```

_NOTE:_ all Liqo resources (i.e. CRDs) will not be automatically purged, so you will not lose your discovered clusters. If you want to delete these resources after uninstallation, invoke the same script with the `--deleteCrd` flag set.

### Purge all Liqo data

If you want all Liqo resources to be completely purged, add the `--deleteCrd` flag to the script invocation:

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash -s -- --uninstall --deleteCrd
```

### What happens to my deployed applications?

During the uninstallation procedure, the home cluster *de-peers* from each peered cluster, hence giving up to the foreign used resources. Nonetheless, the offloaded applications are automatically rescheduled on the local cluster: you will see them running locally in a few minutes.
