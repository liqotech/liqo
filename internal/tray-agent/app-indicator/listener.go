package app_indicator

import "github.com/liqotech/liqo/internal/tray-agent/agent/client"

//Listener is an event listener that can react calling a specific callback.
type Listener struct {
	//Tag specifies the type of notification channel on which it listens to
	Tag client.NotifyChannel
	//StopChan lets control the Listener event loop
	StopChan chan struct{}
	//NotifyChan is the notification channel on which it listens to
	NotifyChan chan string
}

//newListener returns a new Listener.
func newListener(tag client.NotifyChannel, rcv chan string) *Listener {
	l := Listener{StopChan: make(chan struct{}, 1), Tag: tag, NotifyChan: rcv}
	return &l
}

//Listener returns the registered Listener for the specified NotifyChannel. If such Listener does not exist,
//present == false.
func (i *Indicator) Listener(tag client.NotifyChannel) (listener *Listener, present bool) {
	listener, present = i.listeners[tag]
	return
}

//Listen starts a Listener for a specific channel, executing callback when a notification arrives.
func (i *Indicator) Listen(tag client.NotifyChannel, notifyChan chan string, callback func(objName string, args ...interface{}), args ...interface{}) {
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
