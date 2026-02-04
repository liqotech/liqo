FROM alpine/k8s:1.31.3

# Copy CRD files
COPY deployments/liqo/charts/liqo-crds/crds/*.yaml /crds/

# Copy the run script
COPY build/liqo-crd-upgrade/run.sh /run.sh
RUN chmod +x /run.sh

# Set the entrypoint
ENTRYPOINT ["/run.sh"]
