---
title: Liqo Dashboard
weight: 4
---

LiqoDash is a web-based UI for [Liqo](https://liqo.io).
It allows you to visualize and control all the Liqo components, as well as manage the status of the Liqo system itself.
You can use LiqoDash to get a general overview of your Liqo resources (both local and remote), see the connected and available peers and start new peerings.
Finally, it allows also to create and modify CRDs using either a built-in YAML/JSON editor or a dynamically generated CRD wizard.

LiqoDash also provides information about the state of _your_ cluster and _all_ other clusters you are connected to.

![Dashboard-UI example](../../images/dashboard/dashboard-ui.png)

## Installing LiqoDash
LiqoDash is installed by default during the installation of Liqo and it is bound to the API server of the cluster where Liqo has been installed.

## Accessing LiqoDash

### NodePort
In order to access the dashboard you need to first get the port on which LiqoDash is exposed, which can be done with the following command:
```
kubectl -n liqo describe service liqo-dashboard | grep NodePort
```
Which will output:
```
Type:          NodePort
NodePort:      https  32421/TCP
```
In this case, the dashboard has been exposed to the port ``32421``
Now, you can access LiqoDash using your master node IP and the service port you just found: ``https://<MASTER_IP>:<LIQODASH_PORT>``

**NOTE:** to get your master node IP, you can run ``kubectl get nodes -o wide | grep master``, and take the
``INTERNAL-IP``

### Port-Forward
A simple way to access the dashboard with a simpler URL than the one specified above is to use ``kubectl port-forward``.
```
kubectl port-forward -n liqo service/liqo-dashboard 6443:443
```
To access LiqoDash you can go to:
```
https://localhost:6443
```
**NOTE:** The PORT exposed with the port forward, that in the example is ``6443``, can be any PORT that is not already used.

### Ingress
LiqoDash can also be exposed using an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress).
You must have an [Ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/)
for an Ingress resource to work. If you have one, you can specify the desired host name exporting the variable `DASHBOARD_INGRESS` **before**
installing Liqo, for example:
```
export DASHBOARD_INGRESS=example.liqodash.com
```
Once installed, you can see that an Ingress resource is created:
```
kubectl get -n liqo ingress
```
You can access the dashboard through the provided host name.

### Security certificates
For security reasons, LiqoDash runs over HTTPS, which means that a _Certificate_ is needed to establish the TLS connection.
An x509 self-signed Certificate is generated when the dashboard is deployed, and **must be accepted** by your browser in order to establish
the TLS connection.

The first time you access LiqoDash, it is likely that your browser will show you a warning
that you are trying to access a website that uses a self-signed certificate, and thus it is not trusted
by default. Simply accept the risk and continue to access the dashboard.

If you are exposing LiqoDash through an Ingress, you can either use a certificate provided by a public trusted Certificate Authority (like 
[Let's encrypt](https://letsencrypt.org/)) or a self-signed one.

### Differences between access methods
#### When to use Port-Forward
Using `kubectl port-forward` to access the dashboard service is usually meant for testing/debugging purposes. It is not a 
long term solution, as it require to always run the port-forward and keep it active. However, if you just want to access
the dashboard via localhost, this is the easiest method.
#### When to use NodePort
Using a NodePort means that a specific port is opened on all the Nodes, and traffic sent to this port is forwarded
to a service (in this case the dashboard). It is not recommended to use this method to directly expose a service
on the internet, but if you are going to use the dashboard on the same machine your cluster is, that is a simple
and valid solution.
#### When to use Ingress
Using Ingress is probably the best way to expose LiqoDash, especially if you want to access it through the internet. 
It can provide load balancing, SSL termination and name-based hosting, letting you access the dashboard using
a host name instead of just its IP. Because there many types of Ingress controllers, each one with different
capabilities, it may require some work to properly set up.

## Authentication

### Login with Bearer Token
For security, LiqoDash requires a Bearer Token to log in.
This token is stored within a _secret_, which is created by default during the installation, along with a _service account_ and a _cluster role binding_.

To find the token to use to log in, run the following:
```
kubectl -n liqo describe secret $(kubectl -n liqo get secret | grep liqodash | awk '{print $1}')
```
which should print something like this:
```
Name:         liqodash-admin-sa-token-94v8x
Namespace:    liqo
Labels:       <none>
Annotations:  kubernetes.io/service-account.name: liqodash-admin-sa
              kubernetes.io/service-account.uid: ad421b68-7ca5-4f2b-9022-454eb42f880d

Type:  kubernetes.io/service-account-token

Data
====
ca.crt:     1025 bytes
namespace:  4 bytes
token:      eyJhbGciOiJSUzI1NiIsImtpZCI6Img0d3lBRjRGWTlYbndfcnZFeWNQR2VQX3dRVjhfXzBVLTdlTG95Tm9QMW8ifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJsaXFvIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImxpcW9kYXNoLWFkbWluLXNhLXRva2VuLTk0djh4Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6ImxpcW9kYXNoLWFkbWluLXNhIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiYWQ0MjFiNjgtN2NhNS00ZjJiLTkwMjItNDU0ZWI0MmY4ODBkIiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmxpcW86bGlxb2Rhc2gtYWRtaW4tc2EifQ.ZX4SgxepLjDWMYtvlWUfR3Qjzhf80Jq-17JzF7DSZVMJKvqgah0JIG9Ieqj6DBQr-0xxWnTW6hosNjcdf6pm62SbuiSMwyE3xS_j3dAmCQHx5umGbnTjp6GUaMu8JiFtajOpU7-9f06W5g4I44LF1-3FwgG3OY6vVdL6CypWfjumwgh_yLKE9h7tjKl8CiSfNuLVDWHL4l07W9fEeed8lNmFg4FlvOVHmFglTjz20VKEeu964pNlgK0MRGo_cVnDJyWl7cdeEmR0qfiPup5AMQLWUvlX9RTB7UTiRyw9YYZXPrsX5sdUMVuWb-G9ZQ8eABQI7BAs4uCouuoWmIDzag
```

Now, (1) copy the token, (2) paste it in the login screen, and (3) sign in.

**NOTE:** the LiqoDash Service Account has admin privileges. To find out more about permissions in Kubernetes
read the official [authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/)
and [authorization](https://kubernetes.io/docs/reference/access-authn-authz/authorization/) documentation.

### Login with OIDC
If you have an OIDC provider available in your cluster (such as Keycloak), you can use it to access the dashboard and bypass the standard login with the token.
In order to bind the dashboard with you OIDC provider, you have to edit the dashboard _configmap_, which represents the structure that contains the Dashboard configuration.
To do so, run the following command:
```
kubectl edit -n liqo configmap liqo-dashboard-configmap
```
You should see a YAML representation of the configmap, such as the following:
```
# Please edit the object below. Lines beginning with a '#' will be ignored,
# and an empty file will abort the edit. If an error occurs while saving this file will be
# reopened with the relevant failures.
#
apiVersion: v1
...
  name: liqo-dashboard-configmap
  namespace: liqo
  resourceVersion: "012345"
  selfLink: /api/v1/namespaces/liqo/configmaps/liqo-dashboard-configmap
  uid: 8e48f478-993d-11e7-87e0-901b0e532516
data:
  oidc_client_id: ""
  oidc_client_secret: ""
  oidc_provider_url: ""
  oidc_redirect_uri: ""
kind: ConfigMap
```
Change the values under ``data`` and save the file.
Then, restart the dashboard:
```
kubectl rollout restart deployment liqo-dashboard -n liqo
```
From now on, any attempt to login will be redirected to your OIDC provider for the authentication.

**NOTE:** you can always revert to the token login authentication by editing the configmap.

## Metrics Integration
One of the key feature of LiqoDash is to show the real time status of your cluster (in terms of cpu and memory), as well as the other clusters you
are connected to. In order to show the **actual** consumption of resources you need to have a metrics server running in
your cluster. Otherwise, LiqoDash will consider every resource consuming so much cpu and memory as specified in the _request_ parameter of that resource
(if specified).

You can install a metrics server following the official [documentation](https://github.com/kubernetes-sigs/metrics-server#deployment).
The easiest way to check if `metrics-server` is installed and working properly is to run `kubectl top pod` or `kubectl top node`.

# The views

In the sections below, we are going to describe thoroughly each view in the dashboard and their use.

## Home View
When accessing the dashboard, the first view presented is the **Home**. This page contains general information about
the status of your cluster and your Liqo resources, such as clusters you are connected to and the available ones.
It is divided in four major sections, each of them will be discussed in detail.

![Dashboard-UI example](./../../images/dashboard/dashboard-ui-2.png)

### Header
This first section is mean to show the status of the Liqo system and its working mode.
- Cluster Name: the name of your cluster. Can be modified at any time just by clicking on it.
- Status: it can be `Running` or `Stopped`.
- Cluster ID: a unique identifier of your cluster.
- Mode: in this dropdown menu you can choose between various working modes that Liqo supports. For more informations
about it, refer to the [official documentation](https://doc.liqo.io/user/liqo-brief/#working-modes).
- Policies a link to the Liqo policies page, where you can set your preferred policies.
- Settings: a link to the Liqo configuration page, where you can set your preferred configuration for your system.

### Connected Peers
In this section, as the title states, you can see the clusters you are connected to. 
For each peering it is displayed the _direction_ of the connection, that let you know if you are connected to the
foreign cluster, if they are connected you or both. 

Having established a connection doesn't mean that you are actually
sharing resources, but just that you can. To show if resources are actually shared between you and the foreign
cluster, either offering and/or consuming, connections are colored in different ways:
- `BLUE`: if there is actual resource sharing
- `GRAY`: if there is no resource sharing

If there is resource sharing, the amount of memory consumed in a cluster, in relationship to how much is made available
to share, can be seen around the interested cluster, and hovering over it with the mouse will show the actual percentage
of memory consumed.

For example, in the picture above, the first peering represent a bidirectional connection where both the home cluster
and the foreign one are using resources of each other. In the second case, we can see that the connection is only from
the home cluster to the foreign, meaning that you could use resources of the foreign cluster, but they can not. Also
the connection is `gray`, meaning that there is no actual resource sharing.

#### Details
You can access the details of a particular peering selecting it from the dropdown menu placed right beside the name of
the foreign cluster you are connected to. 

![Dashboard-UI details](./../../images/dashboard/dashboard-details.png)

This view displays some general informations about the peering, such as the direction of the connection, and some more in
depth details about the status of both the home and the foreign cluster. In particular we can see:
- Two double gauge-shaped progress bars that represent how much CPU and RAM is both **consumed** and **reserved** in
percentages and in units (___m___ for CPU and ___Mi___ for memory);
- A table that lists all the PODs offloaded to the other's cluster (in case you are on the _Foreign_ tab) or the PODs
that the one you are connected to is offloading to your cluster (in the _Home_ tab), along with the status of each POD;

##### How metrics are calculated
Metrics are calculated in different ways:
- The _Reserved_ percentage is the sum of each POD's requested resource allocation, in relationship to how
many shared resources are established during the peering. These resources are **guaranteed** to be available. If a
POD (or one of its containers) does not specify requested resources will always be considered as if it has **zero**
resources reserved.
- The _Consumed_ percentage is the sum of each POD's used resource over the total of shared resources agreed. Informations
about the real usage of cpu and memory are given by a metrics server deployed in the cluster. If you do not have one, 
you might want to get one to have precise metrics displayed in the dashboard (you can look at 
[metrics server](https://github.com/kubernetes-sigs/metrics-server#deployment)). If there is no metrics server available
the _Consumed_ metrics will be the same as the _Reserved_ ones, as the dashboard assumes you are consuming as much as
requested.

#### Properties
This menu shows the Liqo resources associated with a specific peer (either available or connected):
- The `Foreign Cluster`: it is the resource that describes the _peer_ and contains informations like the cluster ID or
 the status of the peering. It is always present. For more information, please read the [official documentation](); 
- The `Advertisement`: to put it simply, it describes what the other cluster is offering in terms of harware capabilities
 or software. For more information, please read the [official documentation]();
- The `Peering Request`: For more information, please read the [official documentation]();

For every resource you can modify its `spec` directly in the view (the `status` and `metadata` are readonly, as the
user is not supposed to modify them);

#### Disconnect
You can disconnect from a peer just by clicking `Disconnect` in the peer's menu or selecting the connection
and clicking the `Disconnect` button.

### Available Peers
In this section are listed the peers that are available to connect. You can see some general information regarding the
peer, such as its name or if the peer its connected to your LAN or to an external network.
Clicking on the peer (or the dropdown menu in the right) will let you see more details about the available peer and 
the possible connection.

#### Properties
Very much like the properties we can see on a connected peer, it shows the Liqo resources associated to the available
peer. In that way you can see more informations about the foreign cluster and what it has to offer, and evaluate the
connection with it.

#### Connect
If you want to connect to an available peer, you can do so just by clicking the `Connect` button or select `Connect`
in the peer's menu. This will trigger the connection process, and its various phases will be decribed as a side note
in the peering's section. You can also decide to stop the connection process clicking the `Stop Connecting` button.

### Cluster Status
This view is meant to show the total of resources consumed in either your cluster and all the foreign clusters you are
connected to. Here are displayed, for both _Home_ and _Foreign_ clusters:
- Consumption of CPU and RAM: this donut chart shows you the percentage of resource consumed;
    - In the _Home_ cluster section, it shows your consumption as well as every other peer consumption. Hovering 
    over each slice of the chart will pop-up a tooltip that let you know the peer that is using your resources and 
    the percentage of use.
    - In the _Foreign_ clusters section, it shows how much resources you are using on the clusters you are connected to.
     Hovering over each slice of the chart will pop-up a tooltip that let you know the peer you are using their resources
     from and the percentage of use.
- Consumption trend: a chart that diplays the difference in consumption of CPU and RAM over time.
It is updated every 30 seconds.

Unlike the _Connection Detail_ view (which tells all the details about a single connection), the _Cluster Status_ 
view is a more generic exposure of how and who is using resources on your cluster, as well as how much you are using
others'.

## Custom Resource Definition List View
This view shows a list (or rather, a table) of all the Custom Resource Definitions stored in your cluster. Selecting
a CRD will get you to its specific page.

![Dashboard-UI CRD list](./../../images/dashboard/dashboard-CRD-list.png)

This page is designed to give the user a quick and easy way to explore its CRD. Some general informations are displayed
for each CRD, such as:
- `Kind`: the kind of object this CRD describes.
- `Description`: an optional (yet suggested) parameter of the CRD, which is a brief summary of what the resource is.
- `Group`: the API group that CRD belongs to.
- `Favourite` column: a selectable field that you can use to mark your resource as favourite. A _favourite_ resource
will be added to the sidebar, under the _Favourites_ menu.

### Using filters
Every column of the table comes with a filtering option next to its name. Simply clicking its relative search icon will 
open a pop-up that will let you input your search query.
You can filter the CRDs by:
- `Kind`: you can also just input its partial name.
- `Group`: select all the CRDs that belongs to a group or a sub-group.
- `Description`: comes in handy if you want to list all the resources that fits a particular need. This is of course the
weakest filter, because the description field is an optional parameter of the CRD, and depends on its creator.
- `Favourite`: sort your CRDs, showing your favourites one first.

The number of resources listed each page can also be selected (you can choose between 10, 20, 50 and 100 CRD per page) 

## Single CRD View
This is the view specific to a single Custom Resource Definition. We can see an example below.

![Dashboard-UI CRD](./../../images/dashboard/dashboard-CRD.png)

The header of the view shown the `Kind` property of the CRD, its `Group` and its related `Description`. It is also possible
to mark the resource as `Favourite`, checking the _star_ next to the name. It also let the user perform extra actions,
such as:
- Adding or removing the CRD from a `Custom View` (we will talk about Custom Views later)
- Creating a new Custom Resource related to the CRD.

The body of the CRD view is divided in three sections that you can choose and will display different details of the CRD:
- `Resources` (default): show all the Custom Resources related to the CRD.
- `Annotation`: show the list of annotations for the CRD (if there are any).
- `Schema`: show the JSON of the CRD's validation schema.

### Adding a Custom Resource
You can create a new Custom Resource of the type of the CRD you are viewwing clicking the 
![new CR button](./../../images/dashboard/dashboard-new-CRD-button.png) `new resource` button in the far right of the
header. This will open a drawer where you can select two methods of creation:
- `JSON/YAML`: using this built-in editor you can create a Custom Resource from scratch using JSON or YAML. Note that
using this method will grant only syntax validation, meaning that will notify you if you have made error writing JSON or
YAML, but there are no checks against the CRD schema (that will be performed by Kubernetes regardless). 
- `Form Wizard`: this is one of the key feature of LiqoDash. An auto-generated form based on the CRD validation schema
that will grant you a far more enjoyable user experience when it comes to creating (or editing) resources.  You won't
have to worry about which parameters to insert in the creation of your Custom Resource, because it is all already done.
Just compile the form and click `Submit`. If there are errors (e.g. you are forgetting to fill a required parameter), it
will be notified without having to first go through the Kubernetes validations, saving time.

![Dashboard-UI new CR](./../../images/dashboard/dashboard-new-CR.png)

## Custom Resource
Under the _Resource_ tab of a CRD you can see the list of the Custom Resources associated with it.
Expanding one of them will look something like this.

![Dashboard-UI CR](./../../images/dashboard/dashboard-CR.png)

As you can see, each CustomResource is divided in various sections (the ones with the tool icon next to it):
- `Metadata`: data that helps uniquely identify the object, including a `name` string, `UID`, and optional `namespace`.
You can not modify its parameters.
- `Spec`: describe the _desired_ state of the resource and is based on the CRD schema. It can be modified at any moment.
- `Status`: describes the _current state_ of the object, supplied and updated by the Kubernetes system and its components.
You can not modify it.

For every Custom Resource you can also perform the following actions:
- Editing the resource: clicking the _edit_ button will open a drwaer where you can modify the `spec` of your resource.
- Showing the JSON: clicking on the `JSON` button will show the `spec` and the `status` of the resource in a JSON format.
- Deleting the resource: clicking the ![delete button](./../../images/dashboard/dashboard-delete-button.png) `delete` 
button will prompt a pop-up asking if you are sure of your decision. Clicking `Yes` will delete the resource. **There 
is no undo**.

### Editing the Custom Resource
There are two methods you can use to edit your resource.
- If you just need to quickly edit a parameter, you can do it directly in the form view, clicking the 
![edit CR viewer button](./../../images/dashboard/dashboard-edit-viewer-button.png) `edit` button right
next to the parameter you want to modify. This will allow you to interact with the field and change its value. To save
your changes, just click the `Save changes` button at the end of the form.
- If you want a more complete editing experience, you can click the 
![edit CR button](./../../images/dashboard/dashboard-edit-CR-button.png) `edit` button next to the Custom Resource's 
name. This will open a drawer where you can select two methods of update (very much like the one we saw earlier in the
_resource creation_ section):
    - `JSON`: using this built-in editor you can update a Custom Resource modifying its JSON. 
    Using this method will grant only syntax validation, but there are no checks against the CRD schema 
    (still performed by Kubernetes). Only the `Spec` is displayed.
    - `Form Wizard`: the same form you see when you create a resource, but filled with the data from the `Spec` of the
    resource. Like the previous method, only the `Spec` is displayed.

#### Differences between the two methods
The first method is clearly more direct and best suited for a quick and simple edit of the Custom Resource. You can not
add or remove an element, but you can just update the existing ones.
The second method is more advanced and let you create new objects or delete fields.

## Settings
LiqoDash lets you edit the **Liqo cluster configuration** directly in the `Settings` page. Just like any other resource, you
can view and edit it as you like. For a better user experience it is divided between the various configuration that you
can explore.

More information about the **cluster configuration** can be found [here](https://doc.liqo.io/user/configure/cluster-config/).

## Custom View
LiqoDash offers the user to create its own custom views. A `Custom View` is a page that displays a set of CRDs of your
choise, useful to keep track of different resources at the same time, without the need to switch from one page to another.

![Dashboard-UI CV](./../../images/dashboard/dashboard-custom-view.png)

As you can see in the image above, we have together in a single view three different CRDs.
Every CRD inside the view has its own layout that can be changed at any time. 
Some of the feature of a `Custom View` are:
- Draggable CRD: you can move the CRD by dragging it by the _handle_ located at the top-right of the component and placing
it where you find more suited. The CRDs are automatically packed vertically.
- Static CRD: you can _pin_ the CRD to its location by clicking the pin icon next to the drag handle. Doing so, the CRD
will remain in place and can not be moved either by a direct interaction (using the drag handle) or indirect (moving other
CRDs over it). Useful when you want to keep a CRD always in sight.
- Resizable CRD: you can resize each CRD using the _resize handle_ in the bottom-right corner of the component.

Every change is automatically saved, so every time you access a `Custom View` you can expect every CRD in the same position
as you left the last time.

The view is responsive and support separate layouts per responsive breakpoint, that means that if you resize your window
(changing breakpoint) the new layout will be saved as a different one and will not overwrite the previous.
You can always access the single CRD by clicking on its name.

One of the key feature of LiqoDash, as we have said many times, is the real-time reaction to cluster events. That means that
if something is changed on the cluster it is automatically reflected on the view you are on, if we are interested on it.
The `Custom View` is no exception, so if a CRD that is part of the set of CRDs that we are showing in the view, or any of 
its related resources, is updated, you will se the changes without the need to refresh the page. This applies also if
the layout is changed or if a CRD is added/deleted from the set of CRDs in the `Custom View`.

### Create a new Custom View
Creating a `Custom View` can be done in three ways:
- From the _Sidebar_: clicking the `+ New Custom View` will open a menu like the one in the image below. Here you can specify
a name for your `Custom View` as well as choose the CRDs you want to include with a convenient selector. `Custom View` created
this way will be placed in the `default` namespace.
- From the _single CRD view_: clicking the ![layout-button](./../../images/dashboard/dashboard-layout-button.png) `layout` button 
will pop-up a dropdown menu. Selecting `+ New Custom View`
will open the same menu specified in the first method, but this time the CRD you are on will be already inserted to the
set of CRDs you want in your `Custom View`. As said previously, creating a `Custom View` this way will be place it in 
the `default` namespace.
- From the `View` CRD page: because every `Custom View` is, in fact, just a resource of type `View`, you can create one
as you would do with every other Custom Resource, clicking the ![new CR button](./../../images/dashboard/dashboard-new-CRD-button.png)
`new resource` button on its view. This way you have more freedom
in the creation of your resource (you can choose the `Metadata` of your resource, while in the other methods these parameters
were generated automatically). You can also choose an alternative name for the CRDs you are showing in the `Custom View`.

![Dashboard-UI new-CV](./../../images/dashboard/dashboard-new-CV.png)

### Adding/removing a CRD in a Custom View
While on the page of a CRD, you can select the ![layout-button](./../../images/dashboard/dashboard-layout-button.png) `layout` button. 
This will show a dropdown menu where you can see all the
`Custom View` in your cluster. If the name of a `Custom View` is **red** it means the CRD is already in that `Custom View`.
- Clicking on a **neutral** `Custom View` will add the current CRD to the selected `Custom View`.
- Clicking on a **red** `Custom View` will remove the current CRD from the selected `Custom View`.

As we said earlier, a `Custom View` is nothing more than a resource of type `View`. As such, you can modify it at any time
in the `View` CRD page. You can add or remove a CRD in the editor.

### Delete a Custom View
Deleting a `Custom View` is the same as deleting a resource of type `View`. Just go in the `View` CRD page and delete
the resource clicking the ![delete button](./../../images/dashboard/dashboard-delete-button.png) `delete` button
of the `Custom View` you want to remove.
