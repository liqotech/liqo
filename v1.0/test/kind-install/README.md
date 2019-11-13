# Kind install

## Download and Install kind
If you don't yet have kind installed, you can download and install it followind istruction in [Kind Site](https://kind.sigs.k8s.io/)

## Create Clusters
You can proceed to create clusters once you have `kind` available in your path.
The command for create a cluster is: 

```bash
kind create cluster --config cluster.yaml --name clustername
```

In file cluster.yaml you can specify configuration for clusters.

### Run Script

Run the following command to create `2` `kind` clusters:

```bash
./scripts/create-clusters.sh
```

The `NUM_CLUSTERS` is `2` by default. Set that variable before invoking the
script if you'd like to change the default:

```bash
NUM_CLUSTERS=<num> ./scripts/create-clusters.sh
```

With param KIND_IMAGE="image" or KIND_TAG="tag" you can use a custom image.

(In script there are other some param for customize solution)

## Delete Clusters
The command for delete a cluster is: 

```bash
kind delete cluster --name clustername
```

### Run Script

Run the following command to delete `2` `kind` clusters:

```bash
./scripts/delete-clusters.sh
```

The `NUM_CLUSTERS` is `2` by default. Set that variable before invoking the
script if you'd like to change the default:

```bash
NUM_CLUSTERS=<num> ./scripts/delete-clusters.sh
```