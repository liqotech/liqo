package app_indicator

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

/*----- StatRun -----*/

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
	//ConsumePeerings returns the number of active peerings where the Home cluster
	//is consuming foreign resources.
	ConsumePeerings() int
	//IncConsumePeerings adds a consuming peering to the Indicator status.
	//The operation is allowed only when Liqo is in AUTONOMOUS mode.
	IncConsumePeerings()
	//DecConsumePeerings removes a consuming peering from the Indicator status.
	DecConsumePeerings()
	//OfferPeerings returns the number of active peerings where the Home cluster is
	//sharing its own resources.
	OfferPeerings() int
	//IncOfferPeerings adds an offering peering to the Indicator status.
	//When Liqo is not in AUTONOMOUS mode, there can be at most 1 offering peering.
	IncOfferPeerings()
	//DecOfferPeerings removes an offering peering from the Indicator status.
	DecOfferPeerings()
	//ActivePeerings returns the amount of active peerings.
	ActivePeerings() int
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
	//current number of the active peerings used by the user for consuming foreign resources.
	consumePeerings int
	///current number of the active peerings where the user is sharing its own resources.
	offerPeerings int
	//mutex for the Status.
	lock sync.RWMutex
}

//IsTetheredCompliant checks if the TETHERED mode is eligible
//accordingly to current status. The result can be used to display information.
func (st *Status) IsTetheredCompliant() bool {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.offerPeerings <= 1 && st.consumePeerings <= 0
}

//User returns the Liqo Name of the home cluster connected to the Agent.
func (st *Status) User() string {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.user
}

//SetUser sets the Liqo Name of the home cluster connected to the Agent.
func (st *Status) SetUser(user string) {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.user = user
}

//Running returns the running status of Liqo.
func (st *Status) Running() StatRun {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.running
}

//SetRunning changes the running status of Liqo. Transition to StatRunOff
//implies the end of all active peerings.
func (st *Status) SetRunning(running StatRun) {
	st.lock.Lock()
	defer st.lock.Unlock()
	if running != st.running {
		if running == StatRunOff {
			st.consumePeerings = 0
			st.offerPeerings = 0
		}
		st.running = running
	}
}

//Mode returns the current working mode of Liqo.
func (st *Status) Mode() StatMode {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.mode
}

//SetMode sets the working mode for Liqo.
//If the operation is not allowed for current configuration, it returns an error.
func (st *Status) SetMode(mode StatMode) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	var err error
	if mode != st.mode {
		//it is always possible to revert back to autonomous mode
		if mode == StatModeAutonomous {
			st.mode = mode
		} else if st.mode == StatModeAutonomous && mode == StatModeTethered {
			if st.consumePeerings == 0 && st.offerPeerings <= 1 {
				st.mode = mode
			} else {
				err = errors.New("tethered not allowed: there are active but not allowed peerings")
			}
		}
	}
	return err
}

//ConsumePeerings returns the number of active peerings where the Home cluster
//is consuming foreign resources.
func (st *Status) ConsumePeerings() int {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.consumePeerings
}

//IncConsumePeerings adds a consuming peering to the Indicator status.
//The operation is allowed only when Liqo is in AUTONOMOUS mode.
func (st *Status) IncConsumePeerings() {
	st.lock.Lock()
	defer st.lock.Unlock()
	if st.mode == StatModeAutonomous {
		st.consumePeerings += 1
	}
}

//DecConsumePeerings removes a consuming peering from the Indicator status.
func (st *Status) DecConsumePeerings() {
	st.lock.Lock()
	defer st.lock.Unlock()
	if st.consumePeerings > 0 {
		st.consumePeerings -= 1
	}
}

//OfferPeerings returns the number of active peerings where the Home cluster is
//sharing its own resources.
func (st *Status) OfferPeerings() int {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.offerPeerings
}

//IncOfferPeerings adds an offering peering to the Indicator status.
//When Liqo is not in AUTONOMOUS mode, there can be at most 1 offering peering.
func (st *Status) IncOfferPeerings() {
	st.lock.Lock()
	defer st.lock.Unlock()
	if st.mode == StatModeAutonomous || st.offerPeerings < 1 {
		st.offerPeerings += 1
	}
}

//DecOfferPeerings removes an offering peering from the Indicator status.
func (st *Status) DecOfferPeerings() {
	st.lock.Lock()
	defer st.lock.Unlock()
	if st.offerPeerings > 0 {
		st.offerPeerings -= 1
	}
}

//GoString produces a textual digest on the main status data managed by
//a Status instance.
func (st *Status) GoString() string {
	st.lock.RLock()
	defer st.lock.RUnlock()
	str := strings.Builder{}
	str.WriteString(fmt.Sprintln(st.user))
	str.WriteString(fmt.Sprintf("Liqo is %v\n", st.running))
	str.WriteString(fmt.Sprintf("%v mode\n", st.mode))
	str.WriteString(fmt.Sprintf("%v consuming peerings\n", st.consumePeerings))
	str.WriteString(fmt.Sprintf("%v offering peerings", st.offerPeerings))
	return str.String()
}

//ActivePeerings returns the amount of active peerings.
func (st *Status) ActivePeerings() int {
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.offerPeerings + st.consumePeerings
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
