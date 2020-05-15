Liqo Agent is a system-tray agent that allows the user to interact with Liqo

### REQUIREMENTS
Liqo is built using [systray](https://github.com/getlantern/systray) which 
relies on ```gtk3```,```libappindicator3``` and ```libwebkit2gtk-4.0-dev```. 
In order to build it, install these dependecies via 

```sudo apt-get install libgtk-3-dev libappindicator3-dev libwebkit2gtk-4.0-dev```

### RUN
Liqo Agent requires a valid kubeconfig file in order to connect to the Kubernetes cluster. You can select a file explicitly with the **kubeconfig** argument:

```./liqo-agent --kubeconfig='path/to/kubeconfig/file'```.

If **kubeconfig** option is missing, the program searches for a kubeconfig file in ```$HOME/.kube/config```.