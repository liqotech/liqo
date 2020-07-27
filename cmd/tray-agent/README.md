Liqo Agent is a system-tray agent that allows the user to interact with Liqo

### REQUIREMENTS
Liqo Agent is built using [systray](https://github.com/getlantern/systray) which 
relies on```gcc```, ```libgtk-3-dev```,```libappindicator3-dev``` and ```libwebkit2gtk-4.0-dev```. 

On Linux Mint, ```libxapp-dev``` is also required.

In order to build it, install these dependencies via 

```sudo apt-get install gcc libgtk-3-dev libappindicator3-dev libwebkit2gtk-4.0-dev```

### RUN
Liqo Agent requires a valid kubeconfig file in order to connect to the Kubernetes cluster. You can select a file explicitly with the **kubeconfig** argument:

```./liqo-agent -kubeconf='path/to/kubeconfig/file'```.

If **kubeconfig** option is missing, the program searches for a kubeconfig file in ```$HOME/.kube/config```.