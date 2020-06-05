# Advertisement management
![architecture](../images/cluster-peering/Architecture.png)

### Outgoing chain
1. The user provides the information to communicate with the foreign cluster
2. The broadcaster reads this information to create a client to the foreign cluster
3. The broadcaster reads the cluster resources and, after applying some policies, creates an Advertisement CR
4. The Advertisement is pushed to the foreign cluster

### Ingoing chain
1. An Advertisement is received from the foreign cluster
2. The Advertisement is checked by a policy block
3. If the Advertisement is accepted it is sent to the controller
4. The controller creates a virtual node with the information taken by the Advertisement

### CRD generation
To modify the Advertisement you need to
1. open the file _api/v1/advertisement_types.go_. Here you have all the `struct` types related to the Advertisement CRD
2. add/modify/delete the fields of `AdvertisementSpec` and/or `AdvertisementStatus`
3. run `make -f scripts/advertisement-operator/Makefile`; this will regenerate the code for the new version of Advertisement

### Test execution with KinD
You can simply test the system using KinD (https://github.com/kubernetes-sigs/kind/). We suggest to copy _test/kind_ folder in your local machine.
Move to this folder and modify the file `kind.sh` with the number of clusters you want to create.
Run the script `./kind.sh <ip_address>`, where `<ip_address>` is the local ip address of your machine (NOT the localhost): it will create, configure and start n clusters with everything you need.

### Run instructions
1. create a configMap for every foreign cluster you want to communicate with. 
Each configMap must have name **foreign-kubeconfig-<foreign_cluster_id>** and contain the kubeconfig of the foreign cluster.
(in _deployments/advertisement-operator_ folder there is a file _foreignKubeconfig_cm.yaml_ already set up, you only need to insert the foreign kubeconfig and cluster_id)
Apply it to the cluster `kubectl apply -f deployments/advertisement-operator/foreignKubeconfig_cm.yaml`
2. create a configMap with your configuration information.
In _deployments/advertisement-operator_ folder there is a file _adv-operator_cm.yaml_ already set up, you only need to insert your cluster ID and some optional network information
Apply it to the cluster `kubectl apply -f deployments/advertisement-operator/adv-operator_cm.yaml`
3. run `make install -f scripts/advertisement-operator/Makefile` on both your home cluster and foreign one. This will install the CRD Advertisement
4. run the advertisement broadcaster, which is in charge of creating an Advertisement CR on each foreign cluster
    - outside a cluster: `go build cmd/advertisement-broadcaster/main.go --cluster-id <cluster_id>`, where <cluster-id> is the cluster ID of your home cluster
    - inside a cluster: run `kubectl apply -f deployments/advertisement-broadcaster/broadcaster_deploy.yaml`
5. run the advertisement controller, which is in charge of reconciling the status of the Advertisement CR and spawning the virtual-kubelet
    - outside a cluster: `go build cmd/advertisement-operator/main.go --cluster-id <cluster_id>`, where <cluster-id> is the cluster ID of your home cluster
    - inside a cluster: run `kubectl apply -f deployments/advertisement-operator/adv_deploy.yaml`
6. repeat all the steps on your foreign cluster so that it starts sending its Advertisements to home cluster