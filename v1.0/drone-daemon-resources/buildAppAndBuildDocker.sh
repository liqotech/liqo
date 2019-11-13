CLUSTER_NAME="${CLUSTER_NAME}"

# echo "Build Application..."
# go build -o app .

# echo "Create image docker..."
# docker build -t drone-daemon-resources:first .

# echo "Load image on kind..."
# kind load docker-image drone-daemon-resources:first --name ${CLUSTER_NAME}

echo "Apply config map..,"
kubectl apply -f config-map.yml -n drone

echo "Apply on cluster..,"
kubectl apply -f drone-daemon-resources.yml -n drone

# kubectl run --rm -i drone-daemon-resources --image=drone-daemon-resources:first -n demo
# kubectl apply -f drone-daemon-resources.yml -n drone
# kind load docker-image drone-daemon-resources:first --name cluster1