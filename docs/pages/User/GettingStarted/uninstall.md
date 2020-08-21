---
title: Uninstall Liqo
weight: 4
---

## Uninstall steps

This procedure uninstalls Liqo on your cluster.

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash -s -- --uninstall
```

_NOTE:_ all LIQO resources (i.e. CRDs) will not be purged, so you will not lose your discovered clusters. If you want to delete these resources after uninstallation, call the same script with `--deleteCrd` flag.

### Purge LIQO data

If you want that the installed CRD will be completely purged, add `--deleteCrd` flag to the script.

```bash
curl https://raw.githubusercontent.com/LiqoTech/liqo/master/install.sh | bash -s -- --uninstall --deleteCrd
```

### What happens to my deployed applications?

During uninstall script your cluster does a de-peering from each peered cluster, hence giving up to the foreign used resources. Hence, your offloaded applications will be rescheduled on your local cluster: you will see them running locally in a few minutes.
