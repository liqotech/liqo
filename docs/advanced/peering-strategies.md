# Peering strategies

Liqo provides different strategies to establish a **peer** between clusters, depending on your use case:

## Automatic

This method provides a way to create a **peer** using a single command.

It is the **easiest** way to establish a peering between two clusters but is **less flexible** than the manual method.
All liqo modules are initialized and configured automatically.
Furthermore, it **requires** to have **access to each cluster Kubernetes API Server at the same time**.

This method is ideal for a **simple setup** where you want to establish a peering between two clusters without worrying about the details.

Refer to the [peering](/usage/peer) page for more information.

## Manual on cluster couple

This method allows you to interact with a single **liqo module**.
You will be able to configure each module separately at a more granular level.

However, it **requires** to have **access to both clusters Kubernetes API Server at the same time**.

This method is ideal for a **more complex setup** where you want to establish a peering between two clusters and you need to configure each liqo module in a specific way.

Refer to the [advanced peering](/advanced/manual-peering) section for more information.

## Manual on single cluster

This method is the most **flexible** way to establish a peering between two clusters, but it is also the **most complex**.
We suggest using this method only if you have specific requirements that are not covered by the other methods and if you are familiar with Liqo.

You will learn how to create the **Kubernetes Resources** used by Liqo to setup a peering between two clusters.

It does not require to have access to **both clusters Kubernetes API Server at the same time**.

Refer to the [advanced peering](/advanced/manual-peering) section for more information.
