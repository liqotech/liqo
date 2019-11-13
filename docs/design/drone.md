# drone design

Drone is the module referred [here](README.md) deputed to:
* resources slicing (slicer).
* job scheduling (dronesched).

## Resource slicing

The slicing procedure is devoted to split the available computational resources into slices of variable size. The main goal of a federation cluster is to maximize its profit minimizing the load of its nodes, hence, drone uses an algorithm that dynamizes resource slicing, by tuning the size of the slices to vary of the resources availability and idleness, and the nodes load.

## Job scheduling
![logical-structure](../images/drone/logical_structure.png)

The job scheduling is deputed to schedule the jobs across the cluster federation. It is staged by dronesched, a module that takes federation slices as input, computes the best fitting cluster for its needs and outputs the scheduling outcome. 

1. The deployment_builder fetches the deployment to fulfill by using the deployment getter;
2. the deployment_builder forwards the next job to deploy to the auctioneer;
3. the auctioneer computes the best-fitting cluster for the current job;
4. the auctioneer algorithm runs; it decides that it wants an on-demand auction; it broadcasts the request to the federation by means of the federation_slices_petitioner;
5. the slicer fetches the requested slices by using the federation_slices_getter;
6. The slicer receives its cluster resources by using the resource_getter;
7. the slicer algorithm decides whether to allocate the requested slices;
8. the slicer gives back the new slice (or a NACK response) to the requesting cluster by means of the slices_updater;
9. the auctioneer receives the new slice;
10. the auctioneer algorithm runs again and successes to couple the current job to a cluster;
11. the auctioneer updates the deployment entry and sets the winner cluster by means of the deployment_updater;
12. the deployment manager receives the deployment request;
13. the deployment manager checks whether the requested slice is still available by means of the own_slices_getter;
14. the deployment manager decides whether to host the requested job;
    1.  if not --> it updates the deployment request accordingly; break.
15. the deployment manager sets the requested slice as busy by using own_slices_updater;
16. the deployment manager updates the amount of used resources by means of the resource_updater;
17. the deployment manager updates the deployment request by setting the status to accepted;
18. the deployment builder receives the ACK (or NACK) for the current job; it knows that the job has been offloaded; the auctioneer should evaluate the next job (2).
