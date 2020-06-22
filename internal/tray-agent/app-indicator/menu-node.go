package app_indicator

import (
	"fmt"
	"github.com/getlantern/systray"
	"github.com/ozgio/strutil"
)

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
	nodeIconQuick   = "❱"
	nodeIconAction  = "⬢"
	nodeIconOption  = "\t-"
	nodeIconDefault = ""
	nodeIconChecked = "✔ "
)

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
	// the kill switch to disconnect any event handler connected to the node.
	stopChan chan struct{}
	// flag that indicates whether a Disconnect() operation has been called on the MenuNode
	stopped bool
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
	//text content of the menu item
	title string
}

//newMenuNode creates a MenuNode of type NodeType
func newMenuNode(nodeType NodeType) *MenuNode {
	n := MenuNode{item: systray.AddMenuItem("", ""), nodeType: nodeType}
	n.actionMap = make(map[string]*MenuNode)
	n.optionMap = make(map[string]*MenuNode)
	n.stopChan = make(chan struct{})
	n.SetIsVisible(false)
	switch nodeType {
	case NodetypeQuick:
		n.icon = nodeIconQuick
	case NodetypeAction:
		n.icon = nodeIconAction
	case NodetypeOption:
		n.icon = nodeIconOption
	case NodetypeRoot:
		n.parent = &n
	case NodetypeTitle:
		n.parent = &n
		n.SetIsEnabled(false)
	case NodetypeList:
		n.icon = nodeIconDefault
	default:
		n.icon = nodeIconDefault
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
	if n.stopped {
		n.stopChan = make(chan struct{})
		n.stopped = false
	}
	go func() {
		for {
			select {
			case <-n.item.ClickedCh:
				callback(args...)
			case <-n.stopChan:
				return
			case <-root.quitChan:
				return
			}
		}
	}()
}

//ConnectOnce instantiates a one-time listener for the next 'clicked' event of the node.
func (n *MenuNode) ConnectOnce(callback func(args ...interface{}), args ...interface{}) {
	if n.stopped {
		n.stopChan = make(chan struct{})
		n.stopped = false
	}
	go func() {
		for {
			select {
			case <-n.item.ClickedCh:
				callback(args...)
				return
			case <-n.stopChan:
				return
			case <-root.quitChan:
				return
			}
		}
	}()
}

//Disconnect removes the event handler (if any) from the MenuNode.
func (n *MenuNode) Disconnect() {
	if !n.stopped {
		close(n.stopChan)
		n.stopped = true
	}
}

//------ LIST ------

//UseListChild returns a child LIST MenuNode ready to use.
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

//DisuseListChild mark this LIST MenuNode as unused.
func (n *MenuNode) DisuseListChild() {
	n.isInvalid = true
	n.item.SetTitle("")
	n.SetTag("")
	n.SetIsVisible(false)
}

//------ OPTION ------

//AddOption adds an OPTION to the MenuNode as a choice for the submenu. It is hidden by default.
//
//	- title : label displayed in the menu
//
//	- tag : unique tag for the OPTION
//
//	- callback : callback function to be executed at each 'clicked' event.
//	If callback == nil, the function can be set afterwards using n.Connect()
//	or you can manage the event in your	own loop by retrieving the ClickedChan channel
//	via (*MenuNode).Channel() .
func (n *MenuNode) AddOption(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	o := newMenuNode(NodetypeOption)
	o.SetTitle(title)
	o.SetTag(tag)
	o.parent = n
	n.optionMap[tag] = o
	o.SetIsVisible(false)
	if callback != nil {
		o.Connect(callback, args...)
	}
	return o
}

//Option returns the *MenuNode of the OPTION with this specific tag. If such OPTION does not exist, present == false.
func (n *MenuNode) Option(tag string) (opt *MenuNode, present bool) {
	opt, present = n.optionMap[tag]
	return
}

//options returns the map of all the OPTIONS of the MenuNode.
func (n *MenuNode) options() map[string]*MenuNode {
	return n.optionMap
}

//------ GETTERS/SETTERS ------

//SetTitle sets the text content of the MenuNode label.
func (n *MenuNode) SetTitle(title string) {
	if n.nodeType == NodetypeTitle {
		//the TITLE MenuNode is also used to set the width of the entire menu window
		n.item.SetTitle(strutil.CenterText(title, MenuWidth))

	} else {
		n.item.SetTitle(fmt.Sprintln(n.icon, " ", title))
	}
	n.title = title
}

//Tag returns the MenuNode tag.
func (n *MenuNode) Tag() string {
	return n.tag
}

//SetTag sets the the MenuNode tag.
func (n *MenuNode) SetTag(tag string) {
	n.tag = tag
}

//IsInvalid returns if the content of the LIST MenuNode is no more up to date and has to be refreshed by application
//logic.
func (n *MenuNode) IsInvalid() bool {
	return n.isInvalid
}

//SetIsInvalid change the validity of MenuNode content. If isInvalid==true, the content of the LIST MenuNode is no more
//up to date and has to be refreshed by application logic.
func (n *MenuNode) SetIsInvalid(isInvalid bool) {
	n.isInvalid = isInvalid
}

//IsVisible returns if the MenuNode is currently displayed in the menu.
func (n *MenuNode) IsVisible() bool {
	return n.isVisible
}

//SetIsVisible change the MenuNode visibility in the menu.
func (n *MenuNode) SetIsVisible(isVisible bool) {
	if isVisible {
		n.item.Show()
		n.isVisible = true
	} else {
		n.item.Hide()
		n.isVisible = false
	}
}

//IsEnabled returns if the MenuNode label is clickable by the user (if displayed).
func (n *MenuNode) IsEnabled() bool {
	return !n.item.Disabled()
}

//SetIsEnabled change MenuNode possibility to be clickable.
func (n *MenuNode) SetIsEnabled(isEnabled bool) {
	if isEnabled {
		n.item.Enable()
	} else {
		n.item.Disable()
	}
}

//IsChecked returns if MenuNode has been checked.
func (n *MenuNode) IsChecked() bool {
	return n.item.Checked()
}

//SetIsChecked (un)check the MenuNode.
func (n *MenuNode) SetIsChecked(isChecked bool) {
	if isChecked && !n.item.Checked() {
		n.item.SetTitle(fmt.Sprintf("%s%s", nodeIconChecked, n.title))
		n.item.Check()
	} else if !isChecked && n.item.Checked() {
		strutil.ReplaceAllToOne(n.title, []string{nodeIconChecked}, "")
		n.item.Uncheck()
	}
}

//IsDeactivated returns if the user cannot interact with the MenuItem (isDeactivated).
func (n *MenuNode) IsDeactivated() bool {
	return n.isDeactivated
}

//if isDeactivated==true, the user cannot interact with the MenuItem (isDeactivated).
func (n *MenuNode) SetIsDeactivated(isDeactivated bool) {
	n.isDeactivated = isDeactivated
	n.SetIsEnabled(!isDeactivated)
}
