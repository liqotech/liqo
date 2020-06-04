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
	"fmt"
	bip "github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
	"github.com/netgroup-polito/dronev2/internal/tray-agent/icon"
	"github.com/ozgio/strutil"
)

const MenuWidth = 64

/*NodeType distinguishes different kinds of MenuNodes:

ROOT: root of the Menu Tree

QUICK: simple shortcut to perform quick actions, e.g. navigation commands. It is always visible.

ACTION: launch an application command. It can open command submenu (if present)

OPTION: submenu choice

LIST: placeholder item used to dynamically display application output

TITLE: node with special text formatting used to display menu header
*/
type NodeType int

const (
	NodetypeRoot NodeType = iota
	NodetypeQuick
	NodetypeAction
	NodetypeOption
	NodetypeList
	NodetypeTitle
)

//NodeIcon represents a string prefix helping to graphically distinguish different kinds of Menu entries (NodeType)
type NodeIcon string

const (
	NodeiconQuick   = "❱"
	NodeiconAction  = "⬢"
	NodeiconOption  = "\t-"
	NodeiconDefault = ""
)

type Icon int

const (
	IconLiqoMain Icon = iota
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
}

//GetIndicator initialize and returns the Indicator singleton. This function should not be called before Run()
func GetIndicator() *Indicator {
	if root == nil {
		root = &Indicator{}
		root.menu = newMenuNode(NodetypeRoot)
		root.activeNode = root.menu
		root.icon = IconLiqoMain
		systray.SetIcon(icon.IcoMain)
		root.label = ""
		systray.SetTitle("")
		root.menuTitleNode = newMenuNode(NodetypeTitle)
		systray.AddSeparator()
		root.quickMap = make(map[string]*MenuNode)
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
	switch ico {
	case IconLiqoMain:
		systray.SetIcon(icon.IcoMain)
	default:
		systray.SetIcon(icon.IcoMain)
	}
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

//------ NOTIFICATIONS ------

//todo implement notification logic and settings panel
//Notify manages Indicator notification logic
func (i *Indicator) Notify(title string, message string, icoPath string) {
	_ = bip.Notify(title, message, icoPath)
}

// ****** MENUNODE ******

// MenuNode is a stateful wrapper type that provides a better management of
// systray/MenuItem type and additional features, such as submenus
type MenuNode struct {
	// the getlantern/systray MenuItem actually allocated on the menu. It contains the ClickedChan channel that responds
	// to the 'clicked' event
	item *systray.MenuItem
	// the type of the MenuNode
	nodeType NodeType
	// unique tag of the MenuNode that can be used as a key to get access to it, e.g. using (*Indicator)
	tag string
	// parent MenuNode in the menu tree hierarchy
	parent *MenuNode
	// nodesList stores the MenuNode children of type LIST. The node uses them to dynamically display to the user
	// the output of application functions. Use these kind of MenuNodes by calling (*MenuNode).UseListChild() and
	// (*MenuNode).DisuseListChild() methods:
	//
	// - child1 := node.UseListChild()
	//
	// - child1.DisuseListChild()
	nodesList []*MenuNode
	// current number of LIST MenuNode actually in use (isInvalid == false) with valid content
	listLen int
	// total number of LIST MenuNode allocated by the father MenuNode since Indicator start.
	// Some of the LIST nodes may have their content invalid and have to be refreshed by application logic
	listCap int
	//map that stores ACTION MenuNodes, associating them with their tag. This map is actually used only by the ROOT node.
	actionMap map[string]*MenuNode
	//map that stores OPTION MenuNodes, associating them with their tag. These nodes are used to create submenu choices
	optionMap map[string]*MenuNode
	//if isVisible==true, the MenuItem of the node is shown in the menu to the user
	isVisible bool
	//if isDeactivated==true, the user cannot interact with the MenuItem
	isDeactivated bool
	//if isInvalid==true, the content of the LIST MenuNode is no more up to date and has to be refreshed by application
	//logic
	isInvalid bool
	//text prefix that is prepended to the MenuNode title when it is shown in the menu
	icon string
}

//newMenuNode creates a MenuNode of type NodeType
func newMenuNode(nodeType NodeType) *MenuNode {
	n := MenuNode{item: systray.AddMenuItem("", ""), nodeType: nodeType}
	n.actionMap = make(map[string]*MenuNode)
	n.optionMap = make(map[string]*MenuNode)
	n.SetIsVisible(false)
	switch nodeType {
	case NodetypeQuick:
		n.icon = NodeiconQuick
	case NodetypeAction:
		n.icon = NodeiconAction
	case NodetypeOption:
		n.icon = NodeiconOption
	case NodetypeRoot:
		n.parent = &n
	case NodetypeTitle:
		n.parent = &n
		n.SetIsEnabled(false)
	case NodetypeList:
		n.icon = NodeiconDefault
	default:
		n.icon = NodeiconDefault
	}
	return &n
}

//------ EVENT HANDLING ------

//Channel returns the ClickedChan chan of the MenuNode which reacts to the 'clicked' event
func (n *MenuNode) Channel() chan struct{} {
	return n.item.ClickedCh
}

//Connect instantiates a listener for the 'clicked' event of the node
func (n *MenuNode) Connect(callback func(args ...interface{}), args ...interface{}) {
	go func() {
		for {
			select {
			case <-n.item.ClickedCh:
				callback(args...)
			}
		}
	}()
}

//ConnectOnce instantiates a one-time listener for the next 'clicked' event of the node
func (n *MenuNode) ConnectOnce(callback func(args ...interface{}), args ...interface{}) {
	go func() {
		select {
		case <-n.item.ClickedCh:
			callback(args...)
		}
	}()
}

//------ LIST ------

//UseListChild returns a child LIST MenuNode ready to use
func (n *MenuNode) UseListChild() *MenuNode {
	/*
		The systray api, which this package relies on, does not allow to delete elements from the menu stack.
		Hence, when application logic needs more nodes than the ones already allocated, the method allocates them.
		For the next requests, if the number of nodes is lower, the exceeding nodes are invalidated and hidden
	*/
	if !(n.listLen < n.listCap) {
		ch := newMenuNode(NodetypeList)
		ch.parent = n
		n.nodesList = append(n.nodesList, ch)
		n.listCap++
	}
	child := n.nodesList[n.listLen]
	n.listLen++
	child.isInvalid = false
	return child
}

//DisuseListChild mark this LIST MenuNode as unused
func (n *MenuNode) DisuseListChild() {
	n.isInvalid = true
	n.item.SetTitle("")
	n.SetTag("")
	n.SetIsVisible(false)
}

//------ OPTION ------

//AddOption adds an OPTION to the MenuNode as a choice for the submenu. It is hidden by default.
//
//- title : label displayed in the menu
//
//- tag : unique tag for the OPTION
//
//- callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//afterwards using n.Connect() or you can manage the event in your own loop by retrieving the ClickedChan channel
//via (*MenuNode).Channel()
func (n *MenuNode) AddOption(title string, tag string) *MenuNode {
	o := newMenuNode(NodetypeOption)
	o.SetTitle(title)
	o.SetTag(tag)
	o.parent = n
	n.optionMap[tag] = o
	o.SetIsVisible(false)
	return o
}

//Option returns the *MenuNode of the OPTION with this specific tag. If such OPTION does not exist, present == false
func (n *MenuNode) Option(tag string) (opt *MenuNode, present bool) {
	opt, present = n.optionMap[tag]
	return
}

//options returns the map of all the OPTIONS of the MenuNode
func (n *MenuNode) options() map[string]*MenuNode {
	return n.optionMap
}

//------ GETTERS/SETTERS ------

//SetTitle sets the text content of the MenuNode label
func (n *MenuNode) SetTitle(title string) {
	if n.nodeType == NodetypeTitle {
		//the TITLE MenuNode is also used to set the width of the entire menu window
		n.item.SetTitle(strutil.CenterText(title, MenuWidth))

	} else {
		n.item.SetTitle(fmt.Sprintln(n.icon, " ", title))
	}
}

//Tag returns the MenuNode tag
func (n *MenuNode) Tag() string {
	return n.tag
}

//SetTag sets the the MenuNode tag
func (n *MenuNode) SetTag(tag string) {
	n.tag = tag
}

//IsInvalid returns if the content of the LIST MenuNode is no more up to date and has to be refreshed by application
//logic
func (n *MenuNode) IsInvalid() bool {
	return n.isInvalid
}

//SetIsInvalid change the validity of MenuNode content. If isInvalid==true, the content of the LIST MenuNode is no more
//up to date and has to be refreshed by application logic
func (n *MenuNode) SetIsInvalid(isInvalid bool) {
	n.isInvalid = isInvalid
}

//IsVisible returns if the MenuNode is currently displayed in the menu
func (n *MenuNode) IsVisible() bool {
	return n.isVisible
}

//SetIsVisible change the MenuNode visibility in the menu
func (n *MenuNode) SetIsVisible(isVisible bool) {
	if isVisible {
		n.item.Show()
		n.isVisible = true
	} else {
		n.item.Hide()
		n.isVisible = false
	}
}

//IsEnabled returns if the MenuNode label is clickable by the user (if displayed)
func (n *MenuNode) IsEnabled() bool {
	return !n.item.Disabled()
}

//SetIsEnabled change MenuNode possibility to be clickable
func (n *MenuNode) SetIsEnabled(isEnabled bool) {
	if isEnabled {
		n.item.Enable()
	} else {
		n.item.Disable()
	}
}

//IsChecked returns if MenuNode has been checked
func (n *MenuNode) IsChecked() bool {
	return n.item.Checked()
}

//SetIsChecked (un)check the MenuNode
func (n *MenuNode) SetIsChecked(isChecked bool) {
	if isChecked {
		n.item.Check()
	} else {
		n.item.Uncheck()
	}
}

//IsDeactivated returns if the user cannot interact with the MenuItem (isDeactivated)
func (n *MenuNode) IsDeactivated() bool {
	return n.isDeactivated
}

//if isDeactivated==true, the user cannot interact with the MenuItem (isDeactivated)
func (n *MenuNode) SetIsDeactivated(isDeactivated bool) {
	n.isDeactivated = isDeactivated
	n.SetIsEnabled(!isDeactivated)
}
