# IPAM: ref-counted overlapping reservations and exclusive ownership

## High-Level Features

### Ref-counted overlapping network reservations

Networks can now be acquired multiple times with shared ownership. Each acquisition increments a reference count (`refCount++`); each release decrements it. The network is only freed when `refCount` reaches 0. This allows podCIDR, serviceCIDR, and reserved subnets to overlap without conflict.

### Exclusive network ownership

Networks can be acquired exclusively (`refCount = -1`). An exclusive network blocks all overlapping or child acquisitions on the same prefix. Used for CIDRs that are actively in use by Liqo (externalCIDR, internalCIDR, peering CIDRs).

### Two orthogonal dimensions on the gRPC API

- **`immutable`** (exact prefix only) vs **mutable** (try exact, remap if taken)
- **`exclusive`** (sole ownership) vs **shared** (ref-counted overlapping). Only applies if immutable=true, othwerise it's defaulted to `exclusve`.

These produce 3 valid combinations:

| immutable | exclusive | Use case |
|-----------|-----------|----------|
| true | false | CIDRs that are acquired just to prevent liqo from using it (i.e., blacklisting), hence they don't need exclusive acquire. Examples: podCIDR, serviceCIDR, reserved subnets |
| true | true | CIDRs that are acquired to be actively used by Liqo AND cannot be remapped internally to something. Examples: externalCIDR and internalCIDR if explicitely defined by the user |
| false | true | CIDRs that are acquired to be actively used by Liqo AND can be remapped to something else in case of conflicts. Examples: externalCIDR and InternalCIDR if not defined by the user, CIDRs for peerings |

### Label-driven persistence

Exclusive/shared mode is stored as a label (`ipam.liqo.io/network-shared: "true"`) on Network CRs, so the IPAM can restore the correct mode after restart. Default (no label) = exclusive.

### Dynamic external/internal CIDR allocation

Helm values for `externalCIDR`/`internalCIDR` now default to `""`. When empty, the IPAM allocates a `/16` automatically (mutable + exclusive). When explicitly set, the CIDR is immutable + exclusive.

## Internal Details

### IPAM core (`pkg/ipam/core/`)

- `node.refCount` changed from bool to `int` (-1=exclusive, 0=free, >=1=shared)
- Acquire methods consolidated into 2: `NetworkAcquire(size, exclusive)` and `NetworkAcquireWithPrefix(prefix, exclusive)`
- 4 internal allocation functions consolidated into 2: `allocateNetwork(size, node, exclusive)` and `allocateNetworkWithPrefix(prefix, node, exclusive)`
- IPAM core still allows to allocate with `allocateNetwork(size, node, exclusive=false`, but it's not valid combination from API/gRPC contract.
- Release handles exclusive (sets refCount to 0) vs shared (decrements refCount)

### IPAM server (`pkg/ipam/`)

- wrappers consolidated into: `networkAcquire(prefix)` (mutable, always exclusive) and `networkAcquireSpecific(prefix, exclusive)` (immutable)
- gRPC handler dispatches on the `immutable` flag: immutable calls `networkAcquireSpecific`, mutable calls `networkAcquire`
- `initialize.go` and `sync.go` read the `exclusive` flag from `prefixDetails` to restore the correct mode on restart

### Proto (`pkg/ipam/ipam.proto`)

- Added `exclusive` field (field 4) to `NetworkAcquireRequest`

### Network controller (`pkg/liqo-controller-manager/networking/network-controller/`)

- `getRemappedCIDR` now accepts and passes an `exclusive` bool
- Reconciler reads both `NetworkNotRemapped` and `NetworkIsExclusive` from Network CR labels

### Controller manager initialization (`cmd/liqo-controller-manager/modules/networking.go`)

- `initializeReservedNetworks` hardcoded to `immutable=true, exclusive=false` (case 2 — reserved networks are always shared)

### Helm / values

- `externalCIDR` and `internalCIDR` default to `""` in `values.yaml`
- Template applies `10.70.0.0/16` / `10.80.0.0/16` as fallback defaults when left empty
- Conditional `network-not-remapped` label based on whether the value is explicitly set by user
- `network-shared: "true"` label on podCIDR, serviceCIDR, and reserved subnets

### Consts / utils

- Added `NetworkSharedLabelKey` and `NetworkSharedLabelValue` constants
- Added `NetworkIsExclusive(nw)` helper to `pkg/utils/ipam`
