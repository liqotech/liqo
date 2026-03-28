#!/bin/sh

set -e

echo "Applying CRDs..."
kubectl apply --server-side --force-conflicts -f /crds/
echo "CRDs applied successfully"
