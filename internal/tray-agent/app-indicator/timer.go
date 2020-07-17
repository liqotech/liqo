package app_indicator

import (
	"errors"
	"time"
)

//Timer is a data structure that allows to control a time triggered loop execution of a callback.
type Timer struct {
	//tag is the Timer id
	tag string
	//controller is the channel that lets control the callback execution, allowing or preventing it whether the channel
	//receive a true or false value.
	controller chan bool
	//active defines if the time triggered callback is executed (active = true)
	active bool
	//quitCh is the stop chan used to permanently stop the time loop
	quitCh chan struct{}
}

//SetActive controls the Timer behavior, allowing or not future calls of the associated callback.
func (t *Timer) SetActive(active bool) {
	t.controller <- active
}

//Active returns if the Timer is currently active, i.e. timed calls of the associated callback are allowed.
func (t *Timer) Active() bool {
	return t.active
}

//StartTimer registers a new Timer in charge of controlling the loop execution of callback. The Timer starts
//automatically and can be controlled using (*Timer).SetActive() .
//
//	- tag : Timer id.
//
//	- interval : specifies the time interval after which the callback execution is triggered.
func (i *Indicator) StartTimer(tag string, interval time.Duration, callback func(args ...interface{}), args ...interface{}) error {
	if _, present := i.timers[tag]; present {
		return errors.New("A Timer with the same tag already exists")
	}
	t := &Timer{
		tag:        tag,
		controller: make(chan bool, 2),
		quitCh:     i.quitChan,
		active:     true,
	}
	i.timers[tag] = t
	go func(timer *Timer) {
		for {
			select {
			case <-time.After(interval):
				if timer.active {
					callback(args...)
				}
			case stat, open := <-timer.controller:
				if open {
					timer.active = stat
				}
			case <-timer.quitCh:
				return
			}
		}
	}(t)
	return nil
}

//Timer returns the registered Timer for the specified tag. If such Timer does not exist, present == false.
func (i *Indicator) Timer(tag string) (timer *Timer, present bool) {
	timer, present = i.timers[tag]
	return
}
