# Examples requirements

Before starting the tutorials below, you should ensure the following software is installed on your system:

* [**Docker**](https://www.docker.com/), the container runtime.
* [**Kubectl**](https://kubernetes.io/docs/tasks/tools/#kubectl), the command-line tool for Kubernetes.
* [**curl**](https://curl.se/), to interact with the tutorial applications through HTTP/HTTPS.
* [**KinD**](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), the Kubernetes in Docker runtime.
* [**liqoctl**](/installation/liqoctl.md) command-line tool to interact with Liqo.

The following tutorials were tested on Linux, macOS, and Windows (WSL2 and Docker Desktop).

```{warning}
To prevent issues with tutorials leveraging more than two clusters, on some systems you may need to increase the maximum number of *inotify* watches:

```bash
sudo sysctl fs.inotify.max_user_watches=52428899
sudo sysctl fs.inotify.max_user_instances=2048
```
