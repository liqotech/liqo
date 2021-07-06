// Package ipam contains the IPAM module. It is in charge of:
// 1. Keep track of used networks/IP addresses
// 2. Assign networks (ex. remap a remote cluster network to a new network)
// 3. Assign IP addresses (ex. to service endpoints)
// 4. Notify GW about endpoint IP remapping
package ipam
