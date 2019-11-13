CLUSTER_NAME="${CLUSTER_NAME}"

# echo "Build Application, Create docker..."
# operator-sdk build drone-operator:first

# echo "Load image on kind..."
# kind load docker-image drone-operator:first --name ${CLUSTER_NAME}

echo "Apply config map..,"
echo "Edit config for CLUSTER_NAME"
sed -i 's/{NAME}/'"$CLUSTER_NAME"'/g' config-map.yml
kubectl apply -f config-map.yml -n drone

sed -i 's/'"$CLUSTER_NAME"'/{NAME}/g' config-map.yml

echo "Apply operator and other utils..."
# Setup Service Account
kubectl apply -f deploy/service_account.yaml -n drone
# Setup RBAC
kubectl apply -f deploy/role.yaml -n drone
kubectl apply -f deploy/role_binding.yaml -n drone
# Setup the CRD
kubectl apply -f deploy/crds/drone_v1alpha1_dronefederateddeployment_crd.yaml -n drone
# Deploy the app-operator
kubectl apply -f deploy/operator.yaml -n drone


# Create an DroneFederatedDeployment CR
# echo "Create ad DroneFederatedDeployment"
# kubectl apply -f deploy/crds/app.example.com_v1alpha1_appservice_cr.yaml -n drone