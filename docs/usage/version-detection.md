## Summary
Enables querying a remote Liqo cluster's version without establishing full peering, using only a minimal authentication token.

## Motivation
Before initiating peering, administrators need to check version compatibility between clusters. This feature allows version queries without full peering setup.

## Changes

### Version Query Infrastructure
- Added `QueryRemoteVersion()` function for standalone version queries
- Created `liqo-version` ConfigMap to expose local cluster version
- Set up `liqo-version-reader` ServiceAccount with minimal RBAC permissions
- Added token-based authentication for reading version ConfigMap

### Helper Functions
- `GetLocalVersion()`: Retrieve local cluster version from ConfigMap
- `GetVersionReaderToken()`: Extract token from secret
- `GetRemoteVersionWithToken()`: Query remote version with minimal auth

### Supporting Features
- Version resources auto-created at liqo-controller-manager startup
- ForeignCluster auto-creation for tenant consumers (enables bidirectional detection)
- Comprehensive unit tests (21 test specs)

## Testing
- ✅ All version package unit tests pass (17/17 specs)
- ✅ Tenant controller tests added
- ✅ `make generate` runs successfully
- ✅ RBAC auto-generated correctly

## Usage Example
```bash
# Extract token from consumer cluster
kubectl get secret -n liqo liqo-version-reader-token \
  -o jsonpath='{.data.token}' | base64 -d > token

# Query version from any cluster
kubectl --server=https://consumer-cluster:6443 \
  --token="$(cat token)" \
  --insecure-skip-tls-verify \
  get configmap liqo-version -n liqo \
  -o jsonpath='{.data.version}'
