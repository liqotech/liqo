---
title: Authentication
weight: 1
---

## Disable the authentication

The authentication in Liqo is enabled by default; in some environments, such as playgrounds or development contexts, you
may want to disable it. To do so, use the following command:

```bash
kubectl patch clusterconfig liqo-configuration --patch '{"spec":{"authConfig":{"allowEmptyToken": true}}}' --type 'merge'
```

> __NOTE__: Disabling authentication will automatically accept peering with any other Liqo instances in the network your cluster is exposed to.