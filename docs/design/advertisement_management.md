# Advertisement management
![architecture](../images/cluster-peering/Architecture.png)

## Components

### Broadcaster
The broadcaster is in charge of sending to other clusters the Advertisement CR, containing the resources made available 
for sharing and (optionally) their prices. It reads from a ConfigMap the _foreign cluster_ kubeconfig, which allows it to manage the Advertisement.
After creating it, a **remote watcher** is started, which is a goroutine that watches the Advertisement `Status` on the 
remote cluster. This way, the _home cluster_ can know if its CR has been accepted by the _foreign cluster_ and if the `podCIDR` has been remapped by the network module.

### Controller
The controller is the module that receives Advertisement CRs and creates the virtual nodes with the announced resources. 
Doing so, the remote clusters (emulated by the virtual nodes) are taken into account by the scheduler, which can offload
 the jobs it receives on them.

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
3. run `make -f scripts/advertisement-operator/Makefile`; this will generate the code for the new version of Advertisement
4. run `make -f scripts/advertisement-operator/Makefile manifests`; this will generate the Advertisement CRD

### Manual execution
Here we'll explain how to manually run only the Advertisement management components, if needed for testing purposes