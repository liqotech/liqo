adv_chart
=========
A Helm chart for Kubernetes

Current chart version is `0.1.0`





## Chart Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| advController.foreignClusterID | string | `"clusterID"` |  |
| advController.image.pullPolicy | string | `"IfNotPresent"` |  |
| advController.image.repository | string | `"liqo/advertisement-operator"` |  |
| broadcaster.image.pullPolicy | string | `"IfNotPresent"` |  |
| broadcaster.image.repository | string | `"liqo/advertisement-broadcaster"` |  |
