# Prometheus Metrics

This section presents the metrics exposed by Liqo, using the [Prometheus](https://prometheus.io/) format.

## Scraping metrics

Metrics are scraped depending on how your **Prometheus server(s)** has been configured.
Liqo assumes that you leverage the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) to run Prometheus, hence providing a **ServiceMonitor** resource for each component.

Gathering of Liqo metrics is **disabled** by default.
To allow Prometheus to scrape metrics from Liqo, you should set the `--enable-metrics` *liqoctl* flag during installation (cf. [installation customization options](InstallCustomization)), which enables Liqo to expose metrics, and it creates the proper **ServiceMonitor** and **PodMonitor** resources.

Alternatively, you can customize these settings using **Helm** (refer to the [Install with Helm](InstallationHelm) section for further details).

## Cross-cluster network metrics

These metrics are available for each peered remote cluster, providing statistics about the cross-cluster network interconnections:

- **liqo_peer_receive_bytes_total**: the total number of bytes received from a remote cluster.
- **liqo_peer_transmit_bytes_total**: the total number of bytes transmitted to a remote cluster.
- **liqo_peer_latency_us**: the round-trip (RTT) latency between the local cluster and a remote cluster, in micro seconds, measured by a periodic UDP `ping` between the two Liqo gateways and sent within the Liqo tunnel itself.
- **liqo_peer_is_connected**: boolean keeping the status of the network interconnection between clusters, i.e., whether the peering is established and works properly, derived from the `ping` measurement above.

### Grafana dashboard

We provide a {download}`sample Grafana dashboard </_downloads/grafana/liqonetwork.json>` to monitor the network interconnection of an arbitrary number of Liqo peerings.
As presented in the screenshot below, it includes an overview section presenting the overall cross-cluster throughput, followed by detailed per-peering throughput and latency information.

![Grafana Network Dashboard](/_static/images/usage/prometheus-metrics/network-dashboard.png)
