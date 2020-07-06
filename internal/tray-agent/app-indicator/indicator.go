package app_indicator

import (
	"github.com/liqoTech/liqo/internal/tray-agent/agent/client"
	"github.com/liqoTech/liqo/internal/tray-agent/icon"
	"sync"
)

//standard width of an item in the tray menu
const menuWidth = 64

//Icon represents the icon displayed in the tray bar
type Icon int

//Icon displayed in the tray bar. It is internally mapped into one of the icons in
//github.com/liqoTech/liqo/internal/tray-agent/icon
const (
	IconLiqoMain Icon = iota
	IconLiqoNoConn
	IconLiqoErr
	IconLiqoAdvNew
	IconLiqoAdvAccepted
	IconLiqoNil
)

//Run starts the Indicator execution, running the onReady() function. After Quit() call, it runs onExit() before
//exiting. It should be called at the very beginning of main() to lock at main thread.
func Run(onReady func(), onExit func()) {
	GetGuiProvider().Run(onReady, onExit)
}

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
	//guiProvider to interact with the graphic server
	gProvider GuiProviderInterface
	//controller of all the application goroutines
	quitChan chan struct{}
	//if true, quitChan is closed and Indicator can gracefully exit
	quitClosed bool
	//data struct that controls Agent interaction with the cluster
	agentCtrl *client.AgentController
	//map of all the instantiated Listeners
	listeners map[client.NotifyChannelType]*Listener
}

//Listener is an event listener that can react calling a specific callback.
type Listener struct {
	//Tag specifies the type of notification channel on which it listens to
	Tag client.NotifyChannelType
	//StopChan lets control the Listener event loop
	StopChan chan struct{}
	//NotifyChan is the notification channel on which it listens to
	NotifyChan chan string
}

//newListener returns a new Listener.
func newListener(tag client.NotifyChannelType, rcv chan string) *Listener {
	l := Listener{StopChan: make(chan struct{}, 1), Tag: tag, NotifyChan: rcv}
	return &l
}

//Config returns the Indicator config.
func (i *Indicator) Config() *config {
	return i.config
}

//AgentCtrl returns the Indicator AgentController that interacts with the cluster.
func (i *Indicator) AgentCtrl() *client.AgentController {
	return i.agentCtrl
}

//-----LISTENERS-----

//Listen starts a Listener for a specific channel, executing callback when a notification arrives.
func (i *Indicator) Listen(tag client.NotifyChannelType, notifyChan chan string, callback func(objName string, args ...interface{}), args ...interface{}) {
	l := newListener(tag, notifyChan)
	i.listeners[tag] = l
	go func() {
		for {
			select {
			//exec handler
			case name, open := <-l.NotifyChan:
				if open {
					callback(name, args...)
				}
				//closing application
			case <-i.quitChan:
				return
				//closing single listener. Channel controlled by Indicator
			case <-l.StopChan:
				delete(i.listeners, tag)
				return
			}
		}
	}()
}

//Listener returns the registered Listener for the specified NotifyChannelType. If such Listener does not exist,
//present == false.
func (i *Indicator) Listener(tag client.NotifyChannelType) (listener *Listener, present bool) {
	listener, present = i.listeners[tag]
	return
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
			}
		}
		i.activeNode = i.menu
	}
}

//-----QUICKS-----

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

//Quick returns the *MenuNode of the QUICK with this specific tag. If such QUICK does not exist, present == false.
func (i *Indicator) Quick(tag string) (quick *MenuNode, present bool) {
	quick, present = i.quickMap[tag]
	return
}

//------ GETTERS/SETTERS ------

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

//Disconnect exits all the event handlers associated with any Indicator MenuNode via the Connect() method.
func (i *Indicator) Disconnect() {
	if !i.quitClosed {
		close(i.quitChan)
		i.quitClosed = true
	}
}

//GetIndicator initializes and returns the Indicator singleton. This function should not be called before Run().
func GetIndicator() *Indicator {
	if root == nil {
		root = &Indicator{}
		root.gProvider = GetGuiProvider()
		root.menu = newMenuNode(NodeTypeRoot)
		root.activeNode = root.menu
		root.SetIcon(IconLiqoMain)
		root.SetLabel("")
		root.menuTitleNode = newMenuNode(NodeTypeTitle)
		GetGuiProvider().AddSeparator()
		root.quickMap = make(map[string]*MenuNode)
		root.config = newConfig()
		root.quitChan = make(chan struct{})
		root.listeners = make(map[client.NotifyChannelType]*Listener)
		root.agentCtrl = client.GetAgentController()
		if !root.agentCtrl.Connected() {
			root.NotifyNoConnection()
		}
	}
	return root
}
