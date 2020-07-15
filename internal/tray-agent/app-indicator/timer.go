package app_indicator

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
