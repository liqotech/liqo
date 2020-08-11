---
title: Liqo Agent 
weight: 1
---

* [**Overview**](#overview)
    * [Features](#features)
    * [Limitations](#limitations)
* [**Architecture and workflow**](#architecture-and-workflow)
* [**Implementation**](#implementation)
* [**Working Modes**](#working-modes)
    * [Autonomous](#autonomous)
    * [Tethered](#tethered)

## Overview
**Liqo Agent** is the Liqo component that lets users interact with Liqo in an easy, friendly way.
It represents one of the two main entry points to the Liqo environment, 
along with the [**LiqoDash**](https://github.com/LiqoTech/dashboard).

Liqo relies on several components running in the Kubernetes environment. 
Monitoring and managing the system directly might prove not so effortless for everyone
and require some training in the use of Kubernetes tools, e.g. 
[_kubectl_](https://kubernetes.io/docs/reference/kubectl/overview/).

The _Agent_ application aims at solving the problem, enhancing the Liqo experience of desktop users.
By means of a menu accessible from the tray icon, they get access to a clean interface allowing them
to control the status of their own Liqo and perform simple operations. Moreover, thanks to the 
notification system, one gets quickly informed about the main events, such as
the arrival of a new Advertisement.

### Features
*   **Status information**: get details about Liqo
    * check if Liqo is turned ON/OFF
    * current Liqo [**_Working Mode_**](#working-modes)
*   Manage Liqo Agent **settings**
    * users can turn on/off notifications and choose how they would like to receive them:
        * silent notifications on the tray icon
        * tray icon in combination with desktop banners 
*   **Peerings** and **Advertisements management**: using a '_networks management -like_' interface,
users can read the received Advertisements (offers).

### Limitations
*   The current implementation of the two Liqo _running statuses_ [ON/OFF] distinguishes them on the 
base of:
    * enabled notifications
    * display of the _available peers_ section.

Future implementations will provide the possibility to actually shut down and restart the entire framework.

* Currently, **_Autonomous_** is the only supported working mode. 
For more information, take a look at the [dedicated section below](/architecture/desktop/).
