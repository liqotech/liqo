package app_indicator

import (
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
)

//Listener is an event listener that can react calling a specific callback.
type Listener struct {
	//Tag specifies the type of notification channel on which it listens to
	Tag client.NotifyChannel
	//StopChan lets control the Listener event loop
	StopChan chan struct{}
	//NotifyChan is the client.NotifyChannel on which it listens to
	NotifyChan chan client.NotifyDataGeneric
}

//newListener returns a new Listener.
func newListener(tag client.NotifyChannel) *Listener {
	ch := client.GetAgentController().NotifyChannel(tag)
	if ch == nil {
		panic("Indicator tried to listen to non existing NotifyChannel")
	}
	l := Listener{StopChan: make(chan struct{}, 1), Tag: tag, NotifyChan: ch}
	return &l
}

//Listener returns the registered Listener for the specified NotifyChannel. If such Listener does not exist,
//present == false.
func (i *Indicator) Listener(tag client.NotifyChannel) (listener *Listener, present bool) {
	listener, present = i.listeners[tag]
	return
}

//Listen starts a Listener for a specific channel, executing callback when a notification arrives.
func (i *Indicator) Listen(tag client.NotifyChannel, callback func(data client.NotifyDataGeneric, args ...interface{}), args ...interface{}) {
	l := newListener(tag)
	i.listeners[tag] = l
	go func() {
		for {
			select {
			//exec handler
			case data, open := <-l.NotifyChan:
				/*While the Agent is OFF, the callback is not executed, in order not to update information
				on status and tray menu or trigger notifications.*/
				if open && i.Status().Running() == StatRunOn {
					callback(data, args...)
					//signal callback execution in test mode
					if et, testing := GetGuiProvider().GetEventTester(); testing {
						et.Done()
					}
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
