package install

const (
	// LiqoctlInstallShortHelp contains the short help message for install Liqoctl command.
	LiqoctlInstallShortHelp = "Install Liqo on a selected cluster"
	// LiqoctlInstallLongHelp contains the long help message for install Liqoctl command.
	LiqoctlInstallLongHelp = `Install Liqo on a selected cluster`
	// LiqoctlInstallCommand contains the use command for the Liqo installation command.
	LiqoctlInstallCommand = "install"
	liqoRepo              = "https://helm.liqo.io/"
	liqoHelmConfigPath    = "/tmp/.helmrepo"
	liqoHelmCachePath     = "/tmp/.helmcache"
	liqoChartName         = "liqo"
)
