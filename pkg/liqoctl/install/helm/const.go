package helm

const liqoRepo = "https://helm.liqo.io/"

// LiqoNamespace contains the default namespace for Liqo installation.
const LiqoNamespace = "liqo"
const liqoHelmConfigPath = "/tmp/.helmrepo"
const liqoHelmCachePath = "/tmp/.helmcache"

// LiqoChartFullName indicates the name where the Liqo chart can be retrieved.
const LiqoChartFullName = "liqo/liqo"

// LiqoReleaseName indicates the default release name when installing the Liqo chart.
const LiqoReleaseName = "liqo"
