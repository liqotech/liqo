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
Take a look at the complete API [documentation](https://pkg.go.dev/github.com/liqotech/liqo/internal/tray-agent).

* **_NOTE_**: In order to orchestrate the tray icon and menu, _Agent_ exploits the 
[systray](https://github.com/getlantern/systray) package which has some limitations:
    * Missing sub-menu implementation. _Agent_ overcomes the problem by using native OS graphic windows.
    * The tray menu works as a stack, and the library offers no possibility to delete an Item. 
    _Agent_ partially solves the problem by changing properly the menu items visibility.

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
        * if present, it opens a connection to the home cluster and starts watching a subset of Liqo
        resources.
            * The **ForeignCluster** CRD allows to control the status of a peering, both consuming
            and offering. 
            * Moreover, using its internal links (*Object Reference*s) to the related instances 
            of the **Advertisement** and **PeeringRequest** CRDs, it provides also the possibility of
             monitoring every stage of the connection, e.g. validating the request of a new connection or
            the details of an offering proposal.
    4. the main routine of the **Gui Provider** blocks, and the _Indicator_ waits for events triggered
    both from **users** (mouse click) or **Liqo** (operations on cluster resources).
* When the user quits the _Agent_ (via the "QUIT" button), the _Gui Provider_ main routine exits, 
and a cleaning routine is performed.
