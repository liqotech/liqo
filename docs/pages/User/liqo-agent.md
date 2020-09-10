---
title: Liqo desktop agent
weight: 4
---

## Overview
The **Liqo Agent** is the component that allows desktop users to interact with Liqo in an easy, friendly way.
It represents one of the two main entry points to the Liqo environment, along with the [**LiqoDash**](https://github.com/liqotech/dashboard).

Liqo relies on several components running in the Kubernetes environment.
Monitoring and managing the system directly might prove rather challenging for many users as it requires good knowledge about Kubernetes tools such as `kubectl`.

The _Agent_ application aims to solve this problem, enhancing the Liqo experience of desktop users.
By means of a menu accessible from the tray icon, they get access to a clean interface allowing them to control the status of their own Liqo and perform the most common operations.
Moreover, thanks to the notification system, the user gets quickly informed about the main events, such as the arrival of a new Advertisement.

### Features
* Show Liqo **status information**:
  * show if Liqo is turned ON/OFF
  * show current Liqo [Working Mode](../liqo-brief#working-modes)
* Manage Liqo Agent **settings**:
  * turn on/off notifications and choose how they would like to receive them:
      * silent notifications on the tray icon
      * tray icon in combination with desktop banners
* **Peerings** and **Advertisements management**: using a '_networks management -like_' interface,
users can read the received Advertisements (offers).

### Limitations
1. The current implementation of the two Liqo _running statuses_ [ON/OFF] distinguishes them on the base of:
   * enabled notifications
   * display of the _available peers_ section.
  Future implementations will provide the possibility to actually shut down and restart the entire framework.

2. Currently, the Liqo Desktop agent supports only the _Autonomous_ working mode.

> For more information, take a look at the dedicated [Architecture section](/architecture/desktop).
