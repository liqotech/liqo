# NAT Firewall

In Case a **gateway server** is behind a NAT firewall, the following steps are required to establish the connection:
You can configure the settings for the connection by setting the following *annotations* in the `values.yaml` file or by using the `liqoctl` command line `--set` option:

Under the `networking.gatewayTemplates.server.service.annotations` key, you can set the following annotations:

* **liqo.io/override-address**: the public IP address of the NAT firewall.
* **liqo.io/override-port**: the public port of the NAT firewall.

```{admonition} Tip
In case you need to have multiple gateways behind the same NAT firewall, you need to override the port for each peer using the `--server-port` flag at peering time.
This flag works with `liqoctl peer` and `liqoctl network connect` commands.
```
