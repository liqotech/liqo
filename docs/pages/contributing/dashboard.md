---

---

## Local Installation 
Here we take a look on how to setup LiqoDash in your development environment.

### Requirements
- Node.js 12+ and npm 6+ ([installation with nvm](https://github.com/creationix/nvm#usage))
- Docker 1.25+ ([installation manual](https://docs.docker.com/engine/installation/linux/docker-ce/ubuntu/))

### Local installation
Clone the [repository](https://github.com/liqotech/dashboard/tree/master) and install the dependencies with:
```
npm install
```
In order to use LiqoDash, there has to be a Kubernetes cluster running, possibly with Liqo installed (though it is
not mandatory). To connect your cluster with LiqoDash, you need to provide at least one environment variable 
`APISERVER_URL`, that is the url of your cluster's API server to whom our Kubernetes library interact with.

In order to do so, run the following:
```
export APISERVER_URL="https://<APISERVER_IP>:<APISERVER_PORT>"
```
That way, you will be asked an authentication token when accessing the dashboard.

If you have set up an OIDC provider (such as Keycloack), you can use it to access the dashboard by exporting the
following environment variables:
```
export OIDC_PROVIDER_URL="https://<YOUR_OIDC_PROVIDER>"
export OIDC_CLIENT_ID="<YOUR_CLIENT_ID>"
export OIDC_CLIENT_SECRET="<YOUR_CLIENT_SECRET>"
export OIDC_REDIRECT_URI="http://localhost:8000>"
export APISERVER_URL="https://<APISERVER_IP>:<APISERVER_PORT>"
```
**NOTE: OIDC authentication has higher priority than the one with the authentication token.**

### Running LiqoDash
When you're all set up, just run:
```
npm start
```
Open a browser and access the UI under `localhost:8000`.

### Using Docker
You can also pull the docker image:
```
docker pull liqo/dashboard:latest
```
And then run:
```
docker run --env APISERVER_URL=<APISERVER_IP>:<APISERVER_PORT> -p 8000:80 liqo/dashboard:latest
```
Open a browser and visit `localhost:8000`.

**NOTE: the command above uses the `--env` option to export the env variables needed to run the dashboard.**