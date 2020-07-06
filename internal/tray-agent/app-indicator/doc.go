/*
package app_indicator provides API to install a system tray Indicator and bind it to a menu.
It relies on the github.com/getlantern/systray to display the indicator (icon+label) and perform
a basic management of each menu entry (MenuNode).

The GetIndicator() function returns the Indicator singleton.

The Indicator can:

* add to the menu a MenuNode QUICK (an always visible shortcut to a simple operation)

* add to the menu a MenuNode ACTION (the entry point for a more complex operation which can also require more choices)

* instantiate an event handler (Listener)

* communicate with the user through a notification system that exploits changes of the Indicator icon and desktop banners.

USAGE EXAMPLE:

		//define execution logic
		func onReady(){
			indicator := app_indicator.GetIndicator()
    		indicator.AddQuick("HOME", "Q_HOME", myFunction)
			...
		}

		func main(){
			//start the indicator
			app_indicator.Run(onReady,func() {})
		}

*/
package app_indicator
