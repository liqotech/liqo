# Liqo's Network

Below you will find links to the available topics in this section:

- [Basic](basic.md): This page presents the foundational concepts and general information about the Liqo networking model.
- [Leaf-to-Leaf](leaftoleaf.md): This section covers the details of leaf-to-leaf networking, including how network traffic is routed between leaf nodes.
- [NodePort](nodeport.md): This section describes NodePort services and how they are managed within the Liqo network.

If you want details about each CRDs, you can check:

- [Firewall Configuration](firewallconfiguration.md): This page provides detailed information about each firewall configuration resource.
- [Route Configuration](routeconfiguration.md): This page provides detailed information about each routing configuration resource.
- [IP](ip.md): This page provides detailed information about IP configuration resource and how they work.
- [InternalNode](internalnode.md): This page provides detailed information about the InternalNode resource and its role in the Liqo network.

## Useful debug commands for Liqo networking

- `tcpdump -tnl -i any \<protocol\>`: capture all traffic for a specific protocol (e.g., `tcp`, `udp`, `icmp`, etc.)
- `tcpdump -tnl -i any tcp port 8080`: capture all TCP traffic on port 8080
- `tcpdump -tnl -i any tcp dst port 8080`: capture traffic destined to port 8080
- `conntrack -D`: delete all the conntrack entries
- `conntrack -L`: list all the conntrack entries
- `nft list ruleset`: list all the rules in the nftables
