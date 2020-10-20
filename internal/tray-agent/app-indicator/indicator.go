package app_indicator

import (
	"fmt"
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	"github.com/liqotech/liqo/internal/tray-agent/icon"
	"sync"
)

//standard width of an item in the tray menu
const menuWidth = 64

//Icon represents the icon displayed in the tray bar
type Icon int

//Icon displayed in the tray bar. It is internally mapped into one of the icons in
//github.com/liqotech/liqo/internal/tray-agent/icon
const (
	IconLiqoMain Icon = iota
	IconLiqoNoConn
	IconLiqoOff
	IconLiqoWarning
	IconLiqoOrange
	IconLiqoGreen
	IconLiqoPurple
	IconLiqoRed
	IconLiqoYellow
	IconLiqoCyan
	IconLiqoNil
)

//Run starts the Indicator execution, running the onReady() function. After Quit() call, it runs onExit() before
//exiting. It should be called at the very beginning of main() to lock at main thread.
func Run(onReady func(), onExit func()) {
	GetGuiProvider().Run(onReady, onExit)
}

//Indicator singleton
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
	//TITLE MenuNode used by the indicator to show the menu header
	menuTitleNode *MenuNode
	//title text currently in use
	menuTitleText string
	//STATUS MenuNode used to display status information.
	menuStatusNode *MenuNode
	//map that stores QUICK MenuNodes, associating them with their tag
	quickMap map[string]*MenuNode
	//reference to the node of the ACTION currently selected. If none, it defaults to the ROOT node
	activeNode *MenuNode
	//data struct containing indicator config
	config *config
	//guiProvider to interact with the graphic server
	gProvider GuiProviderInterface
	//data struct containing Liqo Status, used to control the menuStatusNode
	status StatusInterface
	//controller of all the application goroutines
	quitChan chan struct{}
	//if true, quitChan is closed and Indicator can gracefully exit
	quitClosed bool
	//data struct that controls Agent interaction with the cluster
	agentCtrl *client.AgentController
	//map of all the instantiated Listeners
	listeners map[client.NotifyChannelType]*Listener
	//map of all the instantiated Timers
	timers map[string]*Timer
}

//GetIndicator initializes and returns the Indicator singleton. This function should not be called before Run().
func GetIndicator() *Indicator {
	if root == nil {
		root = &Indicator{}
		root.gProvider = GetGuiProvider()
		root.SetIcon(IconLiqoNoConn)
		root.SetLabel("")
		root.menu = newMenuNode(NodeTypeRoot)
		root.activeNode = root.menu
		root.menuTitleNode = newMenuNode(NodeTypeTitle)
		root.menuStatusNode = newMenuNode(NodeTypeStatus)
		GetGuiProvider().AddSeparator()
		root.quickMap = make(map[string]*MenuNode)
		root.quitChan = make(chan struct{})
		root.listeners = make(map[client.NotifyChannelType]*Listener)
		root.timers = make(map[string]*Timer)
		root.config = newConfig()
		root.status = GetStatus()
		root.RefreshStatus()
		root.agentCtrl = client.GetAgentController()
		if !root.agentCtrl.Connected() {
			root.NotifyNoConnection()
		} else {
			root.SetIcon(IconLiqoMain)
		}
	}
	return root
}

//-----ACTIONS-----

//AddAction adds an ACTION to the indicator menu. It is visible by default.
//
//	title : label displayed in the menu
//
//	tag : unique tag for the ACTION
//
//	callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//	afterwards using (*Indicator).Connect() .
func (i *Indicator) AddAction(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	a := newMenuNode(NodeTypeAction)
	a.parent = i.menu
	a.SetTitle(title)
	a.SetTag(tag)
	if callback != nil {
		a.Connect(false, callback, args...)
	}
	a.SetIsVisible(true)
	i.menu.actionMap[tag] = a
	return a
}

//Action returns the *MenuNode of the ACTION with this specific tag. If not present, present = false
func (i *Indicator) Action(tag string) (act *MenuNode, present bool) {
	act, present = i.menu.actionMap[tag]
	return
}

//SelectAction selects the ACTION correspondent to 'tag' (if present) as the currently running ACTION in the Indicator,
//showing its OPTIONS (if present) and hiding all the other ACTIONS. The ACTION must have isDeActivated = false
func (i *Indicator) SelectAction(tag string) *MenuNode {
	a, exist := i.menu.actionMap[tag]
	if exist {
		if i.activeNode == a || a.isDeactivated {
			return a
		}
		i.activeNode = a
		//If there are other actions than the selected one, use WaitGroup to speed GUI mutation up
		otherActions := len(i.menu.actionMap) - 1
		var wgOther sync.WaitGroup
		wgOther.Add(otherActions)
		for aTag, action := range i.menu.actionMap {
			if aTag != tag {
				//recursively hide all other ACTIONS and all their sub-components
				go func(n *MenuNode, wg *sync.WaitGroup) {
					n.SetIsVisible(false)
					//hide all node sub-components
					for _, option := range n.optionMap {
						option.SetIsVisible(false)
					}
					wg.Done()
				}(action, &wgOther)
			} else {
				//recursively show selected ACTION with its sub-components
				action.SetIsVisible(true)
				action.SetIsEnabled(false)
				//OPTIONS are shown by default
				for _, option := range action.optionMap {
					option.SetIsVisible(true)
				}
			}
		}
		wgOther.Wait()
		return a
	}
	return nil
}

//DeselectAction deselects any currently selected ACTION, reverting the GUI to the home page. This does not affect
//potential status changes (e.g. enabled/disabled).
func (i *Indicator) DeselectAction() {
	if i.activeNode != i.menu {
		for _, action := range i.menu.actionMap {
			if action != i.activeNode {
				action.SetIsVisible(true)
			} else {
				action.SetIsVisible(true)
				if !action.isDeactivated {
					action.SetIsEnabled(true)
				}
				//hide all node sub-components
				for _, option := range action.optionMap {
					option.SetIsVisible(false)
				}
				for _, listNode := range action.nodesList {
					listNode.DisuseListChild()
				}
				//temporary workaround for current implementation of "Liqo Peers;
				//the action is automatically managed.
				action.SetIsEnabled(false)
			}
		}
		i.activeNode = i.menu
	}
}

//AddQuick adds a QUICK to the indicator menu. It is visible by default.
//
//	title : label displayed in the menu
//
//	tag : unique tag for the QUICK
//
//	callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//	afterwards using (*MenuNode).Connect() .
func (i *Indicator) AddQuick(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	q := newMenuNode(NodeTypeQuick)
	q.parent = q
	q.SetTitle(title)
	q.SetTag(tag)
	if callback != nil {
		q.Connect(false, callback, args...)
	}
	q.SetIsVisible(true)
	i.quickMap[tag] = q
	return q
}

//-----QUICKS-----

//Quick returns the *MenuNode of the QUICK with this specific tag. If such QUICK does not exist, present == false.
func (i *Indicator) Quick(tag string) (quick *MenuNode, present bool) {
	quick, present = i.quickMap[tag]
	return
}

//-----GRAPHIC METHODS-----

//AddSeparator adds a separator line to the indicator menu
func (i *Indicator) AddSeparator() {
	i.gProvider.AddSeparator()
}

//SetMenuTitle sets the text content of the TITLE MenuNode, displayed as the menu header.
func (i *Indicator) SetMenuTitle(title string) {
	i.menuTitleNode.SetTitle(title)
	i.menuTitleNode.SetIsVisible(true)
	i.menuTitleText = title
}

//Icon returns the icon-id of the Indicator tray icon currently set.
func (i *Indicator) Icon() Icon {
	return i.icon
}

//SetIcon sets the Indicator tray icon. If 'ico' is not a valid argument or ico == IconLiqoNil,
//SetIcon does nothing.
func (i *Indicator) SetIcon(ico Icon) {
	var newIcon []byte
	switch ico {
	case IconLiqoNil:
		return
	case IconLiqoMain:
		newIcon = icon.LiqoMain
	case IconLiqoOff:
		newIcon = icon.LiqoOff
	case IconLiqoNoConn:
		newIcon = icon.LiqoNoConn
	case IconLiqoWarning:
		newIcon = icon.LiqoWarning
	case IconLiqoOrange:
		newIcon = icon.LiqoOrange
	case IconLiqoGreen:
		newIcon = icon.LiqoGreen
	case IconLiqoPurple:
		newIcon = icon.LiqoPurple
	case IconLiqoRed:
		newIcon = icon.LiqoRed
	case IconLiqoYellow:
		newIcon = icon.LiqoYellow
	case IconLiqoCyan:
		newIcon = icon.LiqoCyan
	default:
		return
	}
	i.gProvider.SetIcon(newIcon)
	i.icon = ico
}

//Label returns the text content of Indicator tray label.
func (i *Indicator) Label() string {
	return i.label
}

//SetLabel sets the text content of Indicator tray label.
func (i *Indicator) SetLabel(label string) {
	i.label = label
	i.gProvider.SetTitle(label)
}

//RefreshLabel updates the content of the Indicator label
//with the total number of actual peerings.
func (i *Indicator) RefreshLabel() {
	n := i.status.ActivePeerings()
	if n <= 0 {
		i.SetLabel("")
	} else {
		i.SetLabel(fmt.Sprintf("(%v)", n))
	}
}

//--------------

//Quit stops the indicator execution
func (i *Indicator) Quit() {
	if i != nil {
		i.Disconnect()
		if i.agentCtrl.Connected() {
			i.agentCtrl.StopCaches()
		}
	}
	i.gProvider.Quit()
}

//Disconnect exits all the event handlers associated with any Indicator MenuNode via the Connect() method.
func (i *Indicator) Disconnect() {
	if !i.quitClosed {
		close(i.quitChan)
		i.quitClosed = true
	}
}

//AgentCtrl returns the Indicator AgentController that interacts with the cluster.
func (i *Indicator) AgentCtrl() *client.AgentController {
	return i.agentCtrl
}
