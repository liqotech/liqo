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
			mocked: mockedGui,
		}
	})
	return guiProviderInstance
}

//GuiProviderInterface wraps the methods to interact with the OS graphic server and manage a tray icon with its menu.
type GuiProviderInterface interface {
	//Run initializes the GUI and starts the event loop, then invokes the onReady callback. It blocks until
	//systray.Quit() is called. After Quit() call, it runs onExit() before exiting. It should be called before calling
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
	//AddMenuItem returns an Item, e.g. an entry of the tray menu. The menu works as a stack.
	AddMenuItem(title string) Item
	//Mocked returns whether the interaction with the OS graphic server is mocked.
	Mocked() bool
}

//A guiProvider provides the function to interact with the OS graphic server.
//It can act as a mocked provider if UseMockedGuiProvider() is previously called.
type guiProvider struct {
	//if mocked == true, guiProvider acts a mocked provider
	mocked bool
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

func (g *guiProvider) AddMenuItem(title string) Item {
	if !g.mocked {
		return systray.AddMenuItem(title, "")
	} else {
		return &mockItem{clickChan: make(chan struct{}, 2)}
	}
}

func (g *guiProvider) Mocked() bool {
	return g.mocked
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
}

//mockItem implements a mock github.com/getlantern/systray/MenuItem
type mockItem struct {
	visible   bool
	checked   bool
	disabled  bool
	title     string
	clickChan chan struct{}
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
