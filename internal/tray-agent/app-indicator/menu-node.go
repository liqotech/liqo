package app_indicator

import (
	"fmt"
	"github.com/getlantern/systray"
	"github.com/ozgio/strutil"
	"sync"
)

/*NodeType defines the kind of a MenuNode, each one with specific features.

NodeType distinguishes different kinds of MenuNodes:

		ROOT:	root of the Menu Tree.

		QUICK:	simple shortcut to perform quick actions, e.g. navigation commands. It is always visible.

		ACTION:	launch an application command. It can open command submenu (if present).

		OPTION:	submenu choice.

		LIST:	placeholder item used to dynamically display application output.

		TITLE:	node with special text formatting used to display menu header.

		STATUS:	non clickable node that displays status information.
*/
type NodeType int

//set of defined NodeType kinds
const (
	//NodeTypeRoot represents a NodeType of a ROOT MenuNode: root of the Menu Tree.
	NodeTypeRoot NodeType = iota
	//NodeTypeQuick represents a NodeType of a QUICK MenuNode: simple shortcut to perform quick actions,
	//e.g. navigation commands. It is always visible.
	NodeTypeQuick
	//NodeTypeAction represents a NodeType of an ACTION MenuNode: launches an application command.
	//It can open command submenu (if present).
	NodeTypeAction
	//NodeTypeOption represents a NodeType of an OPTION MenuNode: submenu choice (hidden by default).
	NodeTypeOption
	//NodeTypeList represents a NodeType of a LIST MenuNode: placeholder MenuNode used to dynamically
	//display application output.
	NodeTypeList
	//NodeTypeTitle represents a NodeType of a TITLE MenuNode: node with special text formatting
	//used to display the menu header.
	NodeTypeTitle
	//NodeTypeStatus represents a NodeType of a STATUS MenuNode: non clickable node that displays status information
	//about Liqo.
	NodeTypeStatus
)

//NodeIcon represents a string prefix helping to graphically distinguish different kinds of Menu entries (NodeType).
type NodeIcon string

//literal prefix that can be prepended to a MenuNode title, identifying its NodeType or some feature
const (
	nodeIconQuick   = "❱ "
	nodeIconAction  = "⬢ "
	nodeIconOption  = ""
	nodeIconDefault = ""
	nodeIconChecked = "✔ "
)

// MenuNode is a stateful wrapper type that provides a better management of the
// Item type and additional features, such as submenus.
type MenuNode struct {
	// the Item actually allocated and displayed on the menu stack. It also contains
	//the internal ClickedChan channel that reacts to the 'item clicked' event.
	item Item
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
	// nodeList stores the MenuNode children of type LIST. The node uses them to dynamically display to the user
	// the output of application functions. Use these kind of MenuNodes by calling UseListChild, FreeListChild
	// and FreeListChildren methods:
	//
	//		child1 := node.UseListChild(childTitle,childTag)
	//
	//		node.FreeListChild(childTag)
	//
	//		node.FreeListChildren()
	nodeList *nodeList
	//map that stores ACTION MenuNodes, associating them with their tag. This map is actually used only by the ROOT node.
	actionMap map[string]*MenuNode
	//map that stores OPTION MenuNodes, associating them with their tag. These nodes are used to create submenu choices
	optionMap map[string]*MenuNode
	//if isVisible==true, the MenuItem of the node is shown in the menu to the user
	isVisible bool
	//if isInvalid==true, the content of the LIST MenuNode is no more up to date and has to be refreshed by application
	//logic
	isInvalid bool
	//hasCheckbox determines whether the internal item has an embedded graphic checkbox that can be directly managed
	//by the internal Item. If not, (un)check operations are performed by using MenuNode own implementations.
	hasCheckbox bool
	//text prefix that is prepended to the MenuNode title when it is shown in the menu
	icon string
	//text content of the menu item. This redundancy of information is due to the fact Item does not provide getters
	//for the data.
	title string
	//protection for concurrent access to MenuNode attributes.
	sync.RWMutex
}

//newMenuNode creates a MenuNode of type NodeType
func newMenuNode(nodeType NodeType, withCheckbox bool, parent *MenuNode) *MenuNode {
	n := MenuNode{nodeType: nodeType,
		hasCheckbox: withCheckbox}
	n.actionMap = make(map[string]*MenuNode)
	n.optionMap = make(map[string]*MenuNode)
	n.stopChan = make(chan struct{})
	/* Calls to the GuiProviderInterface differ according to hierarchy level constraints of each nodeType.
	ROOT, QUICK, ACTION and TITLE types are level-0 graphic elements, while OPTION and LIST ones are always nested.
	*/
	if nodeType == NodeTypeOption || nodeType == NodeTypeList {
		if parent == nil {
			panic("attempted creation of nested MenuNode with nil parent")
		}
		n.item = GetGuiProvider().AddSubMenuItem(parent.item, withCheckbox)
	} else {
		n.item = GetGuiProvider().AddMenuItem(withCheckbox)
	}
	n.parent = &n
	switch nodeType {
	case NodeTypeQuick:
		n.icon = nodeIconQuick
	case NodeTypeAction:
		n.icon = nodeIconAction
	case NodeTypeOption:
		n.icon = nodeIconOption
		n.parent = parent
	case NodeTypeRoot:
		n.icon = nodeIconDefault
	case NodeTypeTitle:
		n.SetIsEnabled(false)
		n.icon = nodeIconDefault
	case NodeTypeList:
		n.parent = parent
		n.icon = nodeIconDefault
	case NodeTypeStatus:
		n.icon = nodeIconDefault
		n.SetIsEnabled(false)
	default:
		panic("attempted creation of MenuNode with unknown NodeType")
	}
	n.SetIsVisible(false)
	return &n
}

//------ EVENT HANDLING ------

//Channel returns the ClickedChan chan of the MenuNode which reacts to the 'clicked' event
func (n *MenuNode) Channel() chan struct{} {
	switch n.item.(type) {
	case *systray.MenuItem:
		return n.item.(*systray.MenuItem).ClickedCh
	case *mockItem:
		return n.item.(*mockItem).ClickedCh()
	default:
		return nil
	}
}

//Connect instantiates a listener for the 'clicked' event of the node.
//If once == true, the event handler is at most executed once.
func (n *MenuNode) Connect(once bool, callback func(args ...interface{}), args ...interface{}) {
	n.Lock()
	if n.stopped {
		n.stopChan = make(chan struct{})
		n.stopped = false
	}
	n.Unlock()
	var clickCh chan struct{}
	switch n.item.(type) {
	case *systray.MenuItem:
		clickCh = n.item.(*systray.MenuItem).ClickedCh
	case *mockItem:
		clickCh = n.item.(*mockItem).ClickedCh()
	default:
		clickCh = make(chan struct{}, 2)
	}
	go func() {
		for {
			select {
			case <-clickCh:
				callback(args...)
				if et, testing := GetGuiProvider().GetEventTester(); testing {
					et.Done()
				}
				if once {
					return
				}
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
	n.Lock()
	defer n.Unlock()
	if !n.stopped {
		close(n.stopChan)
		n.stopped = true
	}
}

//------ LIST ------

//ListChild returns a tagged LIST MenuNode from the ones currently in use.
func (n *MenuNode) ListChild(tag string) (child *MenuNode, present bool) {
	n.RLock()
	defer n.RUnlock()
	if n.nodeList == nil {
		return nil, false
	}
	child, present = n.nodeList.usedNode(tag)
	return
}

//UseListChild returns a child LIST MenuNode ready to use and visible to users.
//The node can use them to dynamically display to the user the output of application functions.
func (n *MenuNode) UseListChild(title string, tag string) *MenuNode {
	n.Lock()
	defer n.Unlock()
	if n.nodeList == nil {
		n.nodeList = newNodeList(n)
	}
	return n.nodeList.useNode(title, tag)
}

//FreeListChild marks a LIST MenuNode and its nested children as unused, graphically removing them
//from the submenu of MenuNode n in the tray menu. This is a no-op in case of tagged child missing.
func (n *MenuNode) FreeListChild(tag string) {
	n.RLock()
	defer n.RUnlock()
	nl := n.nodeList
	if nl == nil {
		return
	}
	if node, present := nl.usedNode(tag); present {
		node.FreeListChildren()
		nl.Lock()
		defer nl.Unlock()
		nl.freeNode(tag)
	}
}

//FreeListChildren recursively marks all children LIST MenuNode as unused, graphically removing it
//from the submenu of MenuNode n in the tray menu.
func (n *MenuNode) FreeListChildren() {
	n.RLock()
	defer n.RUnlock()
	/* The recursion of this method is made by a 2-steps process:
	1) MenuNode.FreeListChildren() : exported function that checks nullity of its nodeList;
	2) nodeList.freeAllNodes() : internal function calling FreeListChildren on each LIST child.
	*/
	if n.nodeList == nil {
		return
	}
	n.nodeList.freeAllNodes()
}

//ListChildrenLen returns the number of LIST MenuNode currently in use.
func (n *MenuNode) ListChildrenLen() int {
	n.RLock()
	defer n.RUnlock()
	if n.nodeList == nil {
		return 0
	}
	return n.nodeList.usedNodeLen()
}

//------ OPTION ------

//AddOption adds an OPTION to the MenuNode as a choice for the submenu.
//
//		title : label displayed in the menu
//
//		tag : unique tag for the OPTION
//
//		callback : callback function to be executed at each 'clicked' event. If callback == nil,
//		the function can be set afterwards using n.Connect() .
//
//		withCheckbox : if true, add a graphic checkbox on the menu element.
func (n *MenuNode) AddOption(title string, tag string, tooltip string, withCheckbox bool, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	o := newMenuNode(NodeTypeOption, withCheckbox, n)
	o.SetTitle(title)
	o.SetTag(tag)
	o.SetTooltip(tooltip)
	o.SetIsVisible(true)
	if callback != nil {
		o.Connect(false, callback, args...)
	}
	n.Lock()
	defer n.Unlock()
	n.optionMap[tag] = o
	return o
}

//Option returns the *MenuNode of the OPTION with this specific tag. If such OPTION does not exist, present = false.
func (n *MenuNode) Option(tag string) (opt *MenuNode, present bool) {
	n.RLock()
	defer n.RUnlock()
	opt, present = n.optionMap[tag]
	return
}

//------ GETTERS/SETTERS ------

//SetTitle sets the text content of the MenuNode label.
func (n *MenuNode) SetTitle(title string) {
	n.Lock()
	defer n.Unlock()
	if n.nodeType == NodeTypeTitle {
		//the TITLE MenuNode is also used to set the width of the entire menu window
		n.item.SetTitle(strutil.CenterText(title, menuWidth))

	} else {
		n.item.SetTitle(fmt.Sprintln(n.icon, title))
	}
	n.title = title
}

//Title returns the text content of the menu entry. Eventual check tick for checked MenuNode is not included.
func (n *MenuNode) Title() string {
	n.RLock()
	defer n.RUnlock()
	return n.title
}

//Tag returns the MenuNode tag.
func (n *MenuNode) Tag() string {
	n.RLock()
	defer n.RUnlock()
	return n.tag
}

//SetTag sets the the MenuNode tag.
func (n *MenuNode) SetTag(tag string) {
	n.Lock()
	defer n.Unlock()
	n.tag = tag
}

//SetTooltip sets a 'mouse hover' tooltip for the MenuNode. This is a no-op for Linux builds.
func (n *MenuNode) SetTooltip(tooltip string) {
	n.Lock()
	defer n.Unlock()
	n.item.SetTooltip(tooltip)
}

//IsInvalid returns if the content of the LIST MenuNode is no more up to date and has to be refreshed by application
//logic.
func (n *MenuNode) IsInvalid() bool {
	n.RLock()
	defer n.RUnlock()
	return n.isInvalid
}

//SetIsInvalid change the validity of MenuNode content. If isInvalid==true, the content of the LIST MenuNode is no more
//up to date and has to be refreshed by application logic.
func (n *MenuNode) SetIsInvalid(isInvalid bool) {
	n.Lock()
	defer n.Unlock()
	n.isInvalid = isInvalid
}

//IsVisible returns if the MenuNode is currently displayed in the menu.
func (n *MenuNode) IsVisible() bool {
	n.RLock()
	defer n.RUnlock()
	return n.isVisible
}

//SetIsVisible change the MenuNode visibility in the menu.
func (n *MenuNode) SetIsVisible(isVisible bool) {
	n.Lock()
	defer n.Unlock()
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
	n.RLock()
	defer n.RUnlock()
	return !n.item.Disabled()
}

//SetIsEnabled change MenuNode possibility to be clickable.
func (n *MenuNode) SetIsEnabled(isEnabled bool) {
	n.Lock()
	defer n.Unlock()
	if isEnabled {
		n.item.Enable()
	} else {
		n.item.Disable()
	}
}

//IsChecked returns if MenuNode has been checked.
func (n *MenuNode) IsChecked() bool {
	n.RLock()
	defer n.RUnlock()
	return n.item.Checked()
}

//SetIsChecked (un)check the MenuNode.
func (n *MenuNode) SetIsChecked(isChecked bool) {
	n.Lock()
	defer n.Unlock()
	if isChecked && !n.item.Checked() {
		if !n.hasCheckbox {
			n.item.SetTitle(fmt.Sprintf("%s%s", nodeIconChecked, n.title))
		}
		n.item.Check()
	} else if !isChecked && n.item.Checked() {
		if !n.hasCheckbox {
			n.item.SetTitle(n.title)
		}
		n.item.Uncheck()
	}
}
