package app_indicator

import (
	"github.com/getlantern/systray"
	"sync"
)

//mockedGui controls if Indicator graphic component is mocked (true).
var mockedGui bool

//mockOnce prevents mockedGui to be modified at runtime.
var mockOnce sync.Once

//guiProviderInstance is the guiProvider singleton.
var guiProviderInstance *guiProvider

//guiProviderOnce protects guiProviderInstance.
var guiProviderOnce sync.Once

//UseMockedGuiProvider enables a mocked guiProvider that does not interact with the OS graphic server.
//The real guiProvider internally exploits github.com/getlantern/systray to orchestrate GUI execution.
//
//Function MUST be called before GetGuiProvider in order to be effective.
func UseMockedGuiProvider() {
	mockOnce.Do(func() {
		mockedGui = true
	})
}

//DestroyMockedIndicator destroys the Indicator singleton for
//testing purposes. It works only after calling UseMockedGuiProvider
func DestroyMockedIndicator() {
	if mockedGui {
		root = nil
	}
}

//GetGuiProvider returns the guiProvider singleton that provides the functions to interact with the graphic server.
//
//If UseMockedGuiProvider() has been previously called, it returns a mocked guiProvider.
func GetGuiProvider() GuiProviderInterface {
	guiProviderOnce.Do(func() {
		guiProviderInstance = &guiProvider{
			mocked:      mockedGui,
			eventTester: &EventTester{},
		}
	})
	return guiProviderInstance
}

//GuiProviderInterface wraps the methods to interact with the OS graphic server and manage a tray icon with its menu.
type GuiProviderInterface interface {
	//Run initializes the GUI and starts the event loop, then invokes the onReady callback. It blocks until
	//Quit() is called. After Quit() call, it runs onExit() before exiting. It should be called before
	//any other method of the interface.
	Run(onReady func(), onExit func())
	//Quit exits the GUI runtime execution after Run() has been called.
	Quit()
	//AddSeparator adds a separator bar to the tray menu.
	AddSeparator()
	//SetIcon sets the tray icon.
	SetIcon(iconBytes []byte)
	//SetTitle sets the content of the label next to the tray icon.
	SetTitle(title string)
	/*
		AddMenuItem creates and returns an Item, e.g. an entry of the tray menu. The menu works as a stack with only 'push'
		operation available. Use Item methods (e.g. Item.Hide()) to emulate 'pop' behavior.

			withCheckbox = true has to be used on Linux builds to force the creation of an Item with an actual checkbox.
			Otherwise the graphical behavior of Item.Check() is demanded to internal implementation.
	*/
	AddMenuItem(withCheckbox bool) Item
	/*
		AddSubMenuItem creates and returns a child Item for a parent Item so that it can be displayed as
		a submenu element in the tray menu. Each Item submenu works as a stack with only 'push'
		method available. Use Item methods (e.g. Item.Hide()) to emulate 'pop' behavior.

			withCheckbox = true has to be used on Linux builds to force the creation of an Item with an actual checkbox.
			Otherwise the graphical behavior of Item.Check() is demanded to internal implementation.
	*/
	AddSubMenuItem(parent Item, withCheckbox bool) Item
	//Mocked returns whether the interaction with the OS graphic server is mocked.
	Mocked() bool
	//NewEventTester resets and return the EventTester. You can then call EventTester.Test() to start the testing
	//mechanism for the events handled by the current Indicator instance. Read more on EventTester documentation.
	NewEventTester() *EventTester
	//GetEventTester returns current GuiProvider EventTester instance. Read more on EventTester documentation.
	//
	//If testing==true, the EventTester is currently registering the events handled by the Indicator instance in test mode.
	GetEventTester() (eventTester *EventTester, testing bool)
}

/*EventTester is a WaitGroup-based data struct that enables testers to validate concurrent operations performed
by the callback associated to an Indicator Listener (which reacts to specific events) or a MenuNode (which reacts when
the correspondent graphic menu entry is clicked).

During a test, after calling app-indicator.UseMockedGuiProvider(), you can call GetGuiProvider().NewEventTester() to
create a new EventTester. After calling EventTester.Test(), an EventTester.Done() is called after the execution of the
associated callback of a Listener or a MenuNode (if MenuNode.Connect() has been previously called).

Inside a test, use EventTester.Add() and EventTester.Wait() to synchronize the execution of Listeners callbacks.

	...
	eventTester := GetGuiProvider.NewEventTester()
	eventTester.Test()
	//test changes after a single callback
	eventTester.Add(1)
	// trigger Listener-bind callback
	// wait for callback to return
	eventTester.Wait()
	//perform checks on changes and continue
*/
type EventTester struct {
	sync.WaitGroup
	testing bool
}

func (e *EventTester) Test() {
	e.testing = true
}

//A guiProvider provides the function to interact with the OS graphic server.
//It can act as a mocked provider if UseMockedGuiProvider() is previously called.
type guiProvider struct {
	//if mocked == true, guiProvider acts a mocked provider
	mocked      bool
	eventTester *EventTester
}

func (g *guiProvider) Run(onReady func(), onExit func()) {
	if !g.mocked {
		systray.Run(onReady, onExit)
	}
}

func (g *guiProvider) AddSeparator() {
	if !g.mocked {
		systray.AddSeparator()
	}
}

func (g *guiProvider) Quit() {
	if !g.mocked {
		systray.Quit()
	}
}

func (g *guiProvider) SetIcon(iconBytes []byte) {
	if !g.mocked {
		systray.SetIcon(iconBytes)
	}
}

func (g *guiProvider) SetTitle(title string) {
	if !g.mocked {
		systray.SetTitle(title)
	}
}

func (g *guiProvider) AddMenuItem(withCheckbox bool) Item {
	if !g.mocked {
		if withCheckbox {
			return systray.AddMenuItemCheckbox("", "", false)
		} else {
			return systray.AddMenuItem("", "")
		}
	} else {
		return &mockItem{
			clickChan: make(chan struct{}, 2),
		}
	}
}

func (g *guiProvider) AddSubMenuItem(parent Item, withCheckbox bool) Item {
	if parent == nil {
		panic("invalid creation of child Item with nil parent")
	}
	if !g.mocked {
		parentItem := parent.(*systray.MenuItem)
		if withCheckbox {
			return parentItem.AddSubMenuItemCheckbox("", "", false)
		}
		return parentItem.AddSubMenuItem("", "")
	} else {
		parentItem := parent.(*mockItem)
		return parentItem.AddSubMenuItemCheckbox("", "", false)
	}
}

func (g *guiProvider) Mocked() bool {
	return g.mocked
}

func (g *guiProvider) NewEventTester() *EventTester {
	g.eventTester = &EventTester{}
	return g.eventTester
}

func (g *guiProvider) GetEventTester() (*EventTester, bool) {
	if !g.mocked {
		return g.eventTester, false
	}
	return g.eventTester, g.eventTester.testing
}

//Item is an interface representing the actual item that gets pushed (and displayed) in the stack of the tray menu.
type Item interface {
	//Check checks the Item.
	Check()
	//Uncheck unchecks the Item.
	Uncheck()
	//Checked returns whether the Item is checked.
	Checked() bool
	//Enable enables the Item, making it clickable.
	Enable()
	//Disable disables the Item, preventing it to be clickable.
	Disable()
	//Disabled returns whether the Item is disabled, i.e. not clickable.
	Disabled() bool
	//Show makes the Item visible in the menu.
	Show()
	//Hide hides the Item from the menu.
	Hide()
	//SetTitle sets the content of the Item that will be displayed in the menu.
	SetTitle(title string)
	//SetTooltip sets a tooltip for the Item displayed after a 'mouse hover' event.
	//Currently, this is ineffective on Linux builds.
	SetTooltip(tooltip string)
}

//mockItem implements a mock github.com/getlantern/systray/MenuItem
type mockItem struct {
	visible   bool
	checked   bool
	disabled  bool
	title     string
	tooltip   string
	clickChan chan struct{}
}

func (i *mockItem) SetTooltip(tooltip string) {
	i.tooltip = tooltip
}

//AddSubMenuItem adds an Item as a nested menu entry.
//This method is the mocked counterpart of systray.MenuItem 's method.
func (i *mockItem) AddSubMenuItem(title string, tooltip string) Item {
	return &mockItem{
		title:     title,
		tooltip:   tooltip,
		clickChan: make(chan struct{}, 2),
	}
}

//AddSubMenuItemCheckbox adds an Item as a nested menu entry with a graphic checkbox.
//This method is the mocked counterpart of systray.MenuItem 's method.
func (i *mockItem) AddSubMenuItemCheckbox(title string, tooltip string, checked bool) Item {
	return &mockItem{
		checked:   checked,
		title:     title,
		tooltip:   tooltip,
		clickChan: make(chan struct{}, 2),
	}
}

func (i *mockItem) Check() {
	i.checked = true
}

func (i *mockItem) Uncheck() {
	i.checked = false
}

func (i *mockItem) Checked() bool {
	return i.checked
}

func (i *mockItem) Enable() {
	i.disabled = false
}

func (i *mockItem) Disable() {
	i.disabled = true
}

func (i *mockItem) Disabled() bool {
	return i.disabled
}

func (i *mockItem) Show() {
	i.visible = true
}

func (i *mockItem) Hide() {
	i.visible = false
}

func (i *mockItem) Visible() bool {
	return i.visible
}

func (i *mockItem) SetTitle(title string) {
	i.title = title
}

func (i *mockItem) Title() string {
	return i.title
}

func (i *mockItem) ClickedCh() chan struct{} {
	return i.clickChan
}
