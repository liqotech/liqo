/*package app_indicator provides API to install a system tray Indicator and bind it to a menu.
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

import (
	"github.com/getlantern/systray"
	"github.com/liqoTech/liqo/internal/tray-agent/icon"
)

const MenuWidth = 64

const (
	NodetypeRoot NodeType = iota
	NodetypeQuick
	NodetypeAction
	NodetypeOption
	NodetypeList
	NodetypeTitle
)


type Icon int

const (
	IconLiqoMain Icon = iota
	IconLiqoNoConn
	IconLiqoErr
	IconLiqoAdvNew
	IconLiqoAdvAccepted
	IconLiqoNil
)

//Run starts the indicator execution, running the onReady() function. After Quit() call, it runs onExit() before
//exiting. It should be called at the very beginning of main() to lock at main thread.
func Run(onReady func(), onExit func()) {
	systray.Run(onReady, onExit)
}

//Quit stops the indicator execution
func Quit() {
	systray.Quit()
}

//singleton
var root *Indicator

//Indicator is a stateful data structure that controls the app indicator and its related menu. It can be obtained
//and initialized calling GetIndicator()
type Indicator struct {
	//root node of the menu hierarchy.
	menu *MenuNode
	//indicator label showed in the tray bar along the tray icon
	label string
	//indicator icon-id
	icon Icon
	//special MenuNode of type TITLE used by the indicator to show the menu header
	menuTitleNode *MenuNode
	//title text currently in use
	menuTitleText string
	//map that stores QUICK MenuNodes, associating them with their tag
	quickMap map[string]*MenuNode
	//reference to the node of the ACTION currently selected. If none, it defaults to the ROOT node
	activeNode *MenuNode
	//data struct containing indicator config
	config *config
	quitChan chan struct{}
}

//GetIndicator initialize and returns the Indicator singleton. This function should not be called before Run()
func GetIndicator() *Indicator {
	if root == nil {
		root = &Indicator{}
		root.menu = newMenuNode(NodetypeRoot)
		root.activeNode = root.menu
		root.icon = IconLiqoMain
		systray.SetIcon(icon.LiqoBlack)
		root.label = ""
		systray.SetTitle("")
		root.menuTitleNode = newMenuNode(NodetypeTitle)
		systray.AddSeparator()
		root.quickMap = make(map[string]*MenuNode)
		root.config = newConfig()
		root.quitChan = make(chan struct{})
	}
	return root
}

//-----ACTIONS-----

//AddAction adds an ACTION to the indicator menu. It is visible by default.
//
//- title : label displayed in the menu
//
//- tag : unique tag for the ACTION
//
//- callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//afterwards using (*Indicator).Connect() or you can manage the event in your own loop retrieving the ClickedChan channel
//via (*MenuNode).Channel()
func (i *Indicator) AddAction(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	a := newMenuNode(NodetypeAction)
	a.parent = i.menu
	a.SetTitle(title)
	a.SetTag(tag)
	if callback != nil {
		a.Connect(callback, args)
	}
	a.SetIsVisible(true)
	i.menu.actionMap[tag] = a
	return a
}

//Action returns the *MenuNode of the ACTION with this specific tag. If not present, returns nil
func (i *Indicator) Action(tag string) (act *MenuNode, pres bool) {
	act, pres = i.menu.actionMap[tag]
	return
}

//actions returns the map of all the ACTIONS created since Indicator start
func (i *Indicator) actions() map[string]*MenuNode {
	return i.menu.actionMap
}

//SelectAction selects the ACTION correspondent to 'tag' (if present) as the currently running ACTION in the Indicator,
//showing its OPTIONs (if present) and hiding all the other ACTIONS The ACTION must be isDeActivated == false
func (i *Indicator) SelectAction(tag string) *MenuNode {
	a, exist := i.menu.actionMap[tag]
	if exist {
		if i.activeNode == a || a.isDeactivated {
			return a
		}
		i.activeNode = a
		for aTag, action := range i.menu.actionMap {
			if aTag != tag {
				go func(n *MenuNode) {
					//recursively hide other ACTIONS and all their sub-components
					n.SetIsVisible(false)
					//hide all node sub-components
					for _, option := range n.optionMap {
						option.SetIsVisible(false)
					}
					for _, listNode := range n.nodesList {
						listNode.SetIsVisible(false)
					}
				}(action)
			} else {
				go func(n *MenuNode) {
					a.SetIsEnabled(false)
					//OPTIONs are showed by default
					for _, option := range n.optionMap {
						option.SetIsVisible(true)
					}
					//LIST are directly managed by the ACTION logic and so they are not automatically showed
				}(action)
			}

		}
		return a
	}
	return nil
}

//DeselectAction deselects any currently selected ACTION, reverting the GUI to the home page. This does not affect
//potential status changes (e.g. enabled/disabled)
func (i *Indicator) DeselectAction() {
	if i.activeNode != i.menu {
		for _, action := range i.menu.actionMap {
			if action != i.activeNode {
				action.SetIsVisible(true)
			} else {
				go func(n *MenuNode) {
					n.SetIsVisible(true)
					if !n.isDeactivated {
						n.SetIsEnabled(true)
					}
					//hide all node sub-components
					for _, option := range n.optionMap {
						option.SetIsVisible(false)
					}
					for _, listNode := range n.nodesList {
						listNode.SetIsVisible(false)
						listNode.SetIsInvalid(true)
						listNode.Disconnect()
					}
				}(action)
			}
		}
	}
}

//-----QUICKS-----

//AddQuick adds a QUICK to the indicator menu. It is visible by default.
//
//- title : label displayed in the menu
//
//- tag : unique tag for the QUICK
//
//- callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//afterwards using (*Indicator).Connect() or you can manage the event in your own loop retrieving the ClickedChan channel
//via (*MenuNode).Channel()
func (i *Indicator) AddQuick(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	q := newMenuNode(NodetypeQuick)
	q.parent = q
	q.SetTitle(title)
	q.SetTag(tag)
	if callback != nil {
		q.Connect(callback, args)
	}
	q.SetIsVisible(true)
	i.quickMap[tag] = q
	return q
}

//Quick returns the *MenuNode of the QUICK with this specific tag. If such QUICK does not exist, present == false
func (i *Indicator) Quick(tag string) (quick *MenuNode, pres bool) {
	quick, pres = i.quickMap[tag]
	return
}

//quicks returns the map of all the QUICKS created since Indicator start
func (i *Indicator) quicks() map[string]*MenuNode {
	return i.quickMap
}

//------ GETTERS/SETTERS ------

//AddSeparator adds a separator line to the indicator menu
func (i *Indicator) AddSeparator() {
	systray.AddSeparator()
}

//SetMenuTitle sets the text content of the TITLE MenuNode, displayed as the menu header
func (i *Indicator) SetMenuTitle(title string) {
	i.menuTitleNode.SetTitle(title)
	i.menuTitleNode.SetIsVisible(true)
	i.menuTitleText = title
}

//Menu returns the ROOT node of the menu tree
func (i *Indicator) Menu() *MenuNode {
	return i.menu
}

//Icon returns the icon-id of the Indicator tray icon currently set
func (i *Indicator) Icon() Icon {
	return i.icon
}

//SetIcon sets the Indicator tray icon
func (i *Indicator) SetIcon(ico Icon) {
	i.icon = ico
	var newIcon []byte
	switch ico {
	case IconLiqoNil:
		return
	case IconLiqoMain:
		newIcon = icon.LiqoBlack
	case IconLiqoNoConn:
		newIcon = icon.LiqoNoConn
	case IconLiqoAdvNew:
		newIcon = icon.LiqoOrange
	case IconLiqoErr:
		newIcon = icon.LiqoRed
	case IconLiqoAdvAccepted:
		newIcon = icon.LiqoGreen
	default:
		ico = IconLiqoMain
		newIcon = icon.LiqoBlack
	}
	systray.SetIcon(newIcon)
	root.icon = ico

}

//Label returns the text content of Indicator tray label
func (i *Indicator) Label() string {
	return i.label
}

//SetLabel sets the text content of Indicator tray label
func (i *Indicator) SetLabel(label string) {
	i.label = label
	systray.SetTitle(label)
}

func (i *Indicator) Disconnect() {
/*	for _,q := range i.quickMap{
		q.Disconnect()
	}
	for _,a := range i.actions(){
		a.Disconnect()
	}
*/
}