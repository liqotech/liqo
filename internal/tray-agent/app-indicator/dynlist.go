package app_indicator

import (
	"github.com/oleiade/lane"
	"sync"
)

/*nodeList manages a dynamic list of MenuNode elements OF list NodeType that present themselves as nested items
of a parent MenuNode. It overcomes the GuiProviderInterface main limitation, i.e.
the lack of 'pop' operation from the graphic tray menu stack.
*/
type nodeList struct {
	//parent is the MenuNode owning the nodeList.
	parent *MenuNode
	//usedNodes contains the LIST MenuNode that are currently in use.
	usedNodes map[string]*MenuNode
	//freeNodes is a pool of the LIST MenuNode that have been allocated but not currently in use.
	freeNodes *lane.Queue
	//totFree represents the size of the freeNodes pool.
	totFree int
	//withCheckbox defines if LIST MenuNode elements are provided with a graphic checkbox.
	withCheckbox bool
	//Mutex used to protect operations on the nodeList.
	sync.RWMutex
	//WaitGroup used to parallelize LIST nodes cleaning.
	sync.WaitGroup
}

//newNodeList creates a new nodeList for a parent MenuNode.
func newNodeList(parent *MenuNode) *nodeList {
	if parent == nil {
		panic("attempted creation of nodeList from nil MenuNode parent")
	}
	return &nodeList{
		parent:    parent,
		usedNodes: make(map[string]*MenuNode),
		freeNodes: lane.NewQueue(),
	}
}

//useNode makes available a nested LIST MenuNode, taking it from the freeNodes pool or creating a new one.
func (nl *nodeList) useNode(title string, tag string) *MenuNode {
	nl.Lock()
	defer nl.Unlock()
	var node *MenuNode
	if nl.totFree > 0 {
		node = nl.freeNodes.Dequeue().(*MenuNode)
		nl.totFree--
	} else {
		node = newMenuNode(NodeTypeList, nl.withCheckbox, nl.parent)
	}
	node.SetTitle(title)
	node.SetTag(tag)
	node.SetIsVisible(true)
	nl.usedNodes[tag] = node
	return node
}

//usedNode retrieves, if present, a tagged LIST child in use.
func (nl *nodeList) usedNode(tag string) (node *MenuNode, present bool) {
	nl.RLock()
	defer nl.RUnlock()
	node, present = nl.usedNodes[tag]
	return
}

//freeNode takes a LIST MenuNode away from the ones in use, making it available.
func (nl *nodeList) freeNode(tag string) {
	node, ok := nl.usedNodes[tag]
	if ok {
		node.SetTitle("")
		node.SetTag("")
		node.SetIsVisible(false)
		node.SetIsEnabled(true)
		node.Disconnect()
		delete(nl.usedNodes, tag)
		nl.freeNodes.Enqueue(node)
		nl.totFree++
	}
}

//freeAllNodes iteratively applies freeNode() to all used LIST MenuNode.
func (nl *nodeList) freeAllNodes() {
	nl.Lock()
	defer nl.Unlock()
	defer nl.Wait()
	for tag, node := range nl.usedNodes {
		nl.Add(1)
		go func(n *MenuNode, str string) {
			defer nl.Done()
			n.FreeListChildren()
			nl.freeNode(str)
		}(node, tag)
	}
}
