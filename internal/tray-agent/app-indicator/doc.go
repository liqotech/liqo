/*
package app_indicator provides API to install a system tray Indicator and bind it to a menu.
It relies on the github.com/getlantern/systray to display the indicator (icon+label) and perform
a basic management of each menu entry (MenuItem)

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
