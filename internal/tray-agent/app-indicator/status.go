package app_indicator

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

//StatRun defines the running status of Liqo.
type StatRun bool

const (
	//StatRunOff defines the 'OFF' running status of Liqo.
	StatRunOff StatRun = false
	//StatRunOn defines the 'ON' running status of Liqo.
	StatRunOn StatRun = true
)

const (
	//textual description for the StatRunOff StatRun value.
	statRunOffDescription = "OFF"
	//textual description for the StatRunOn StatRun value.
	statRunOnDescription = "ON"
)

//PeeringType defines a type of peering with a foreign cluster.
type PeeringType bool

const (
	//PeeringIncoming defines a peering where the home cluster shares its own resources with a foreign cluster.
	PeeringIncoming PeeringType = true
	//PeeringOutgoing defines a peering where the home cluster is consuming the resources of a foreign cluster.
	PeeringOutgoing PeeringType = false
)

//String converts in human-readable format the StatRun information.
func (rs StatRun) String() string {
	str := ""
	switch rs {
	case StatRunOff:
		str = statRunOffDescription
	case StatRunOn:
		str = statRunOnDescription
	}
	return str
}

/*----- StatMode -----*/

//StatMode defines the Working Modes for Liqo, i.e. abstraction models designed to cover some common use cases.
//Each Mode represents a set of allowed operations and statuses.
type StatMode int

const (
	/*StatModeAutonomous defines the AUTONOMOUS working mode.

	The device uses its own on-board intelligence, i.e. it connects
	to its local K8s API server and lets the local orchestrator control the scheduling.

	- It can work as a stand-alone cluster, consuming only its own resources

	- It can connect to multiple peers, both consuming (under its control) foreign resources
	and sharing its proprietary resources to other peers.
	The system acts as a set of cooperating nodes, exploiting each foreign cluster's VirtualKubelet
	seen by the local scheduler. Each sharing operation is independent of the others.
	*/
	StatModeAutonomous StatMode = iota
	/* StatModeTethered defines the TETHERED working mode.
	The device can choose to connect to a single foreign Liqo peer
	(e.g. the corporate network), allowing the remote orchestrator to control the usage of its resources.

	When the tethered peering is established, the remote peer, working in Autonomous mode,
	uses its own API Server and takes control of the shared resources.
	Every resource request made by the device is forwarded to the remote peer which will perform a proper scheduling.
	*/
	StatModeTethered
)

const (
	//StatModeAutonomousHeaderDescription is the short description for the AUTONOMOUS mode.
	StatModeAutonomousHeaderDescription = "AUTONOMOUS"
	//StatModeAutonomousBodyDescription is the extended description for the AUTONOMOUS mode.
	StatModeAutonomousBodyDescription = "Be in control of your Liqo: exchange resources with multiple peers"
	//StatModeTetheredHeaderDescription is the short description for the TETHERED mode.
	StatModeTetheredHeaderDescription = "TETHERED"
	//StatModeTetheredBodyDescription is the extended description for the TETHERED mode.
	StatModeTetheredBodyDescription = "Let a remote peer control your Liqo"
	//StatModeUnknownDescription is a fallback description for undefined mode received from the graphic input.
	StatModeUnknownDescription = "unknown"
)

//String converts in human-readable format the StatMode information.
func (sm StatMode) String() string {
	str := ""
	switch sm {
	case StatModeAutonomous:
		str = StatModeAutonomousHeaderDescription
	case StatModeTethered:
		str = StatModeTetheredHeaderDescription
	default:
		str = StatModeUnknownDescription
	}
	return str
}

//GoString implements the fmt.GoStringer interface. This method is used to display
//an extended text format for a StatMode.
func (sm StatMode) GoString() string {
	var str string
	switch sm {
	case StatModeAutonomous:
		str = fmt.Sprintf("%s\n\n%s", StatModeAutonomousHeaderDescription, StatModeAutonomousBodyDescription)
	case StatModeTethered:
		str = fmt.Sprintf("%s\n\n%s", StatModeTetheredHeaderDescription, StatModeTetheredBodyDescription)
	default:
		str = StatModeUnknownDescription
	}
	return str
}

/* ----- STATUS -----*/

//Status singleton instance for the Indicator
var statusBlock *Status

//StatusInterface wraps the methods to manage the Indicator status.
type StatusInterface interface {
	//User returns the Liqo Name of the home cluster connected to the Agent.
	User() string
	//SetUser sets the Liqo Name of the home cluster connected to the Agent.
	SetUser(user string)
	//Running returns the running status of Liqo.
	Running() StatRun
	//SetRunning changes the running status of Liqo. Transition to StatRunOff
	//implies the end of all active peerings.
	SetRunning(running StatRun)
	//Mode returns the current working mode of Liqo.
	Mode() StatMode
	//SetMode sets the working mode for Liqo.
	//If the operation is not allowed for current configuration, it returns an error.
	SetMode(mode StatMode) error
	/*IsTetheredCompliant checks if the TETHERED mode is eligible
	accordingly to current status. The result can be used to display information.

	This method is not to be intended as a preliminary test
	for an actual mode change. In this case you must use SetMode() which exploits a
	"Compare&Change" protection.
	*/
	IsTetheredCompliant() bool
	//IncDecPeerings increments (add = true) or decrements of 1 unit the number of active peerings of type peering.
	//
	//There can be PeeringOutgoing peerings only when Liqo is in StatModeAutonomous mode.
	//
	//There can be at most 1 PeeringIncoming peering when Liqo is not in StatModeAutonomous mode.
	IncDecPeerings(peering PeeringType, add bool)
	//Peerings returns the number of active peerings of type PeeringType.
	Peerings(peering PeeringType) int
	//ActivePeerings returns the amount of active peerings.
	ActivePeerings() int
	//Peers returns the number of Liqo peers discovered by the home cluster and currently available.
	Peers() int
	//IncDecPeers increments (add = true) or decrements the number of available peers.
	IncDecPeers(add bool)
	//GoString produces a textual digest on the main status data managed by
	//a Status instance.
	GoString() string
}

//GetStatus initializes and returns the Status singleton. This function should not be called before Run().
func GetStatus() StatusInterface {
	if statusBlock == nil {
		/*statusBlock boots up with default settings:
			- OFF Liqo status
			- Autonomous mode
		Further changes are up to other Indicator components.*/
		statusBlock = &Status{}
	}
	return statusBlock
}

//Status defines a data structure containing information about the current status of the Liqo instance,
//e.g. if it is running, the selected working mode and a summary of the active peerings.
type Status struct {
	//the LiqoName of the current user.
	user string
	//the running status of the Liqo instance.
	running StatRun
	//the current Liqo working mode.
	mode StatMode
	//total number of discovered peers.
	peers int
	//current number of the active peerings consuming foreign resources.
	outgoingPeerings int
	///current number of the active peerings sharing home resources.
	incomingPeerings int
	//mutex for the Status.
	sync.RWMutex
}

//IsTetheredCompliant checks if the TETHERED mode is eligible
//accordingly to current status. The result can be used to display information.
func (st *Status) IsTetheredCompliant() bool {
	st.RLock()
	defer st.RUnlock()
	return st.incomingPeerings <= 1 && st.outgoingPeerings <= 0
}

//User returns the Liqo Name of the home cluster connected to the Agent.
func (st *Status) User() string {
	st.RLock()
	defer st.RUnlock()
	return st.user
}

//SetUser sets the Liqo Name of the home cluster connected to the Agent.
func (st *Status) SetUser(user string) {
	st.Lock()
	defer st.Unlock()
	st.user = user
}

//Running returns the running status of Liqo.
func (st *Status) Running() StatRun {
	st.RLock()
	defer st.RUnlock()
	return st.running
}

//SetRunning changes the running status of Liqo. Transition to StatRunOff
//implies the end of all active peerings.
func (st *Status) SetRunning(running StatRun) {
	st.Lock()
	defer st.Unlock()
	if running != st.running {
		if running == StatRunOff {
			st.outgoingPeerings = 0
			st.incomingPeerings = 0
		}
		st.running = running
	}
}

//Mode returns the current working mode of Liqo.
func (st *Status) Mode() StatMode {
	st.RLock()
	defer st.RUnlock()
	return st.mode
}

//SetMode sets the working mode for Liqo.
//If the operation is not allowed for current configuration, it returns an error.
func (st *Status) SetMode(mode StatMode) error {
	st.Lock()
	defer st.Unlock()
	var err error
	if mode != st.mode {
		//it is always possible to revert back to autonomous mode
		if mode == StatModeAutonomous {
			st.mode = mode
		} else if st.mode == StatModeAutonomous && mode == StatModeTethered {
			if st.outgoingPeerings == 0 && st.incomingPeerings <= 1 {
				st.mode = mode
			} else {
				err = errors.New("tethered not allowed: there are active but not allowed peerings")
			}
		}
	}
	return err
}

//IncDecPeerings increments (add = true) or decrements of 1 unit the number of active peerings of type peering.
//
//There can be PeeringOutgoing peerings only when Liqo is in StatModeAutonomous mode.
//
//There can be at most 1 PeeringIncoming peering when Liqo is not in StatModeAutonomous mode.
func (st *Status) IncDecPeerings(peering PeeringType, add bool) {
	st.Lock()
	defer st.Unlock()
	if peering == PeeringIncoming {
		if add {
			if st.mode == StatModeAutonomous || st.incomingPeerings < 1 {
				st.incomingPeerings += 1
			}
		} else {
			if st.incomingPeerings > 0 {
				st.incomingPeerings -= 1
			}
		}
	} else if peering == PeeringOutgoing {
		if add {
			if st.mode == StatModeAutonomous {
				st.outgoingPeerings += 1
			}
		} else {
			if st.outgoingPeerings > 0 {
				st.outgoingPeerings -= 1
			}
		}
	}
}

//Peerings returns the number of active peerings of type PeeringType.
func (st *Status) Peerings(peering PeeringType) int {
	st.RLock()
	defer st.RUnlock()
	if peering == PeeringIncoming {
		return st.incomingPeerings
	} else {
		return st.outgoingPeerings
	}
}

//Peers returns the number of Liqo peers discovered by the home cluster and currently available.
func (st *Status) Peers() int {
	st.RLock()
	defer st.RUnlock()
	return st.peers
}

//IncDecPeers increments (add = true) or decrements the number of available peers.
func (st *Status) IncDecPeers(add bool) {
	st.Lock()
	defer st.Unlock()
	if add {
		st.peers++
	} else {
		if st.peers > 0 {
			st.peers--
		}
	}
}

//GoString produces a textual digest on the main status data managed by
//a Status instance.
func (st *Status) GoString() string {
	st.RLock()
	defer st.RUnlock()
	str := strings.Builder{}
	str.WriteString(fmt.Sprintln(st.user))
	str.WriteString(fmt.Sprintf("Liqo is %v\n", st.running))
	str.WriteString(fmt.Sprintf("%v mode\n", st.mode))
	str.WriteString(fmt.Sprintf("consuming from %v peers\n", st.outgoingPeerings))
	str.WriteString(fmt.Sprintf("offering to %v peers", st.incomingPeerings))
	return str.String()
}

//ActivePeerings returns the amount of active peerings.
func (st *Status) ActivePeerings() int {
	st.RLock()
	defer st.RUnlock()
	return st.incomingPeerings + st.outgoingPeerings
}

//Status return the Indicator status.
func (i *Indicator) Status() StatusInterface {
	return i.status
}

//RefreshStatus updates the contents of the STATUS MenuNode and the Indicator Label.
func (i *Indicator) RefreshStatus() {
	i.menuStatusNode.SetTitle(i.status.GoString())
	i.RefreshLabel()
}

//DestroyStatus is a testing function used to refresh the Status component.
func DestroyStatus() {
	if GetGuiProvider().Mocked() {
		statusBlock = nil
	}
}
