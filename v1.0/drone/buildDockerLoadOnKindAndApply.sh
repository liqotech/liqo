CLUSTER_NAME="${CLUSTER_NAME}"

# echo "Create image docker..."
# docker build -t drone-agent:first .

# echo "Load image on kind..."
# kind load docker-image drone-agent:first

echo "Edit yaml CLUSTER_NAME"
sed -i 's/{NAME}/'"$CLUSTER_NAME"'/g' drone-deploy.yaml

echo "Apply on K8S"
kubectl apply -f drone-deploy.yaml -n drone

sed -i 's/'"$CLUSTER_NAME"'/{NAME}/g' drone-deploy.yaml