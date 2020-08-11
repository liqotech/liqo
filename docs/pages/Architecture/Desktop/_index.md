---
title: Liqo Agent 
weight: 1
---

## Architecture and workflow


* **GuiProvider**: It is the component that provides the graphic functions to the Indicator, 
internally exploiting the graphic library. It controls:
    * the tray icon
    * the tray label (simple text near the tray icon)
    * the tray menu (creation/position/visibility of the menu items)
    * the entire runtime execution through a couple of callbacks

* **Indicator**: It is the main block that orchestrates the _Agent_ workflow,
acting as a bridge between the graphic operations and the logic. It takes the name from the 
[libappindicator](https://launchpad.net/libappindicator) library. It is in charge of:
    * managing the items of the tray menu (**MenuNode**s), associating each element with a callback
    * interacting with the Liqo cluster using the **Agent Controller** component. This way it can:
        * update status information
        * listen to specific events (e.g. receiving signals from the _Agent Controller_)
        * send notifications to the users.

* **MenuNode**: It is the component that cooperates with the _Indicator_ in the management of the
tray menu. Each MenuNode has exclusive control on a single menu item and its features:
    * **look**, e.g. visibility, text content, possibility to be clickable
    * **functionality**, managing the connection of the item with a callback

* **Agent Controller**: It is the component that manages _Agent_ interactions with the Liqo cluster.
It embeds all the necessary logic and data structures (like clients and caches) in order to 
operate with the Liqo CRDs, watching relevant events and signaling them to the _Indicator_ Notify system.

![Liqo Agent components](/images/tray-agent/liqo_agent-scheme.png)

## Implementation    
Take a look at the complete api [documentation](https://pkg.go.dev/github.com/liqoTech/liqo/internal/tray-agent).

* **_NOTE_**: In order to orchestrate the tray icon and menu, _Agent_ exploits the 
[systray](https://github.com/getlantern/systray) package which has some limitations:
    * Missing sub-menu implementation. _Agent_ overcame the problem using native OS graphic windows.
    * The tray menu works as a stack, and the library offers no possibility to delete an Item. 
    _Agent_ partially solved the problem changing properly the menu items visibility.

* When _Agents_ starts:
    1. the **Gui Provider** starts and open a connection to the OS graphic server ([X](https://x.org/wiki/)).
    Then it calls the main routine where all the application logic is executed in an event loop.
    2. the **Indicator** boots up:
        * Using the connection provided by the _Gui Provider_, it calls the config routine, creating
        and setting all the graphic components (e.g. the MenuNodes)
        * It starts the _Agent Controller_
        * It loads Liqo configuration and status from the cluster (via **Agent Controller**)
        * It loads _Agent_ settings from the cluster and from a config file on the OS filesystem 
    3. the **Agent Controller**:
        * searches for a valid Client configuration for the home cluster (a _kubeconfig_ file)
        * if present, it opens a connection to the home cluster and starts watching a set of Liqo
        resources.
    4. the main routine of the **Gui Provider** blocks, and the _Indicator_ waits for events triggered
    both from **users** (mouse click) or **Liqo** (operations on cluster resources).
* When the user quits the _Agent_ (via the "QUIT" button), the _Gui Provider_ main routine exits, 
and a cleaning routine is performed.
    
## Working Modes

Using a proper orchestration of the Liqo components, _Agent_ introduces two abstraction models, called
**Working Modes**, designed to cover some common use cases.

### Autonomous

![Autonomous mode](/images/tray-agent/autonomous-mode.png)

In the **_Autonomous_** mode (default), **the device uses its own on-board intelligence**, i.e. it connects to its
local K8s _API server_ and lets the local orchestrator control the scheduling. This way:

1. It can work as a _stand-alone cluster_, **consuming only its own resources**
    * Useful if there is no internet connection or there are no available peers
2. It can connect to multiple peers, both **consuming** (under its control) **foreign resources** 
and **sharing its proprietary resources** to other peers. 
    * The system acts as a set of cooperating nodes, exploiting each foreign cluster's _VirtualKubelet_
    seen by the local scheduler.    
    * Each sharing operation is independent of the others.             

### Tethered

![Tethered mode](/images/tray-agent/tethered-mode.png)

When working in **_Tethered_** mode, the device can **_choose_ to connect** to a **_single_** foreign Liqo peer
(e.g. the corporate network), allowing the remote orchestrator to control the usage of its resources.

* When the tethered peering is established:
    * The device turns off its intelligence
    * The remote peer, working in _Autonomous_ mode, uses its own _API Server_ and takes control
    of the shared resources.
    * Every resource request made by the device is forwarded to the remote peer which will 
    perform a proper scheduling. This way:
        * A device with few resources can leverage additional external power in order to complete its jobs.
        * A corporate may achieve a more efficient resource allocation, easily sharing 
        its computational power and application logic also with remote employees, which may result in 
        a significant cost reduction.
* When the tethered peering ends:
    * the device on-board intelligence takes back control of the local resources      

> **NOTE**: Since the tethering require only one unidirectional peering, transition from _Autonomous_
>to _Tethered_ mode is allowed only in presence of **at most one active connection** where the device is 
>**offering** resources.