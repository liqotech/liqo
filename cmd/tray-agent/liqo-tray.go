package main

import (
	"github.com/getlantern/systray"
	agent "github.com/netgroup-polito/dronev2/internal/tray-agent/agent-client"
	"github.com/netgroup-polito/dronev2/internal/tray-agent/icon"
	"os"
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTitle("Liqo")
	go showMain()
}

// once the menu entries are defined, a valid improvement could be the use a hierarchical system of maps
// and the association of a metadata struct to each MenuItem, in order to implement a better MVC pattern
func showMain() {
	AdvClient, err := agent.CreateClient(agent.AcquireConfig())
	if err != nil {
		os.Exit(1)
	}
	// Workaround to set window width: the tray window's width is the maximum among the ones of the menu entries.
	// the tooltip isn't available for linux version
	m_title := systray.AddMenuItem("                           Liqo Agent                           ", "")
	m_title.Disable()
	m0_back := systray.AddMenuItem("Back", "")
	m0_quit := systray.AddMenuItem("Quit Liqo Agent", "")
	systray.AddSeparator()

	// main feature
	m0_adv := systray.AddMenuItem("Show Advertisements", "")
	m0_adv.Uncheck()
	//placeholders to display requested infos
	m0_adv_1 := systray.AddMenuItem("	- adv1", "")
	m0_adv_1.Hide()
	m0_adv_2 := systray.AddMenuItem("	- adv2", "")
	m0_adv_2.Hide()
	m0_adv_3 := systray.AddMenuItem("	- adv3", "")
	m0_adv_3.Hide()
	// demo 1: it is possible to display a submenu
	m0_lab_visual := systray.AddMenuItem("Toggle Sub Menu Visibility", "")
	m0_lab_visual.Uncheck()
	m1_0_label := m0_lab_visual.AddSubMenuItem("	- submenu entry", "")
	m1_0_label.Hide()
	// demo 2: it is possible to enable and disable a menu entry
	m0_lab_available := systray.AddMenuItem("Toggle Entry Availability", "")
	m0_lab_available.Uncheck()
	m1_1_label := m0_lab_available.AddSubMenuItem("entry", "")
	m1_1_label.Disable()
	//

	//control loop that handles 'item clicked' events
	for {
		select {
		case <-m0_adv.ClickedCh:
			if m0_adv.Checked() {
				m0_adv.Uncheck()
				m0_adv.SetTitle("Show Advertisements")
				m0_adv_1.Hide()
				m0_adv_2.Hide()
				m0_adv_3.Hide()
			} else {
				m0_adv.Check()
				m0_adv.SetTitle("Hide Advertisements")
				//liqo logic here
				advList, err := agent.ListAdvertisements(&AdvClient)
				if err != nil {
					m0_adv_1.SetTitle("Agent could not connect to the cluster")
					m0_adv_1.Show()
				} else {
					//iteration is not here performed since there is not yet a dynamic and centralized
					// menuItem management
					if len(advList) > 0 {
						m0_adv_1.SetTitle(advList[0])
					}
					m0_adv_1.Show()
					m0_adv_2.Show()
					m0_adv_3.Show()
				}
			}

		case <-m0_lab_visual.ClickedCh:
			m0_lab_visual.SetTitle("Sub Menu Example")
			m0_lab_visual.Disable()
			m1_0_label.Show()
			//
			m0_adv.Hide()
			m0_adv_1.Hide()
			m0_adv_2.Hide()
			m0_adv_3.Hide()
			//
			m0_lab_available.Hide()
			m1_1_label.Hide()


		case <-m0_lab_available.ClickedCh:
			if m0_lab_available.Checked() {
				m0_lab_available.Uncheck()
				m1_1_label.Disable()
			} else {
				m0_lab_available.Check()
				m1_1_label.Enable()
			}
		case <-m0_back.ClickedCh:
			m0_adv.SetTitle("Show Advertisements")
			m0_adv.Show()
			m0_adv.Uncheck()
			m0_adv_1.SetTitle("")
			m0_adv_1.Hide()
			m0_adv_2.Hide()
			m0_adv_3.Hide()
			m0_lab_visual.SetTitle("Toggle Sub Menu Visibility")
			m0_lab_visual.Enable()
			m0_lab_visual.Show()
			m0_lab_visual.Uncheck()
			m1_0_label.Hide()
			m0_lab_available.Show()
			m0_lab_available.Uncheck()
			m1_1_label.Show()
			m1_1_label.Disable()

		case <-m0_quit.ClickedCh:
			systray.Quit()
			return
		}
	}

}

func onExit() {
}
