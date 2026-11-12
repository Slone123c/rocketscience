package main

import (
	"embed"
	"fmt"
	"log"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"rocketscience/internal/core"
	"rocketscience/internal/network"
)

//go:embed assets/tray.png
var trayAsset embed.FS

// Desktop owns the macOS menu bar item and the windows.
type Desktop struct {
	mu             sync.Mutex
	pageBusy       bool
	mainBounds     *mygo.Rectangle
	mainIdle       pageIdle
	panelIdle      pageIdle
	idleStop       chan struct{}
	service        *core.Service
	profiles       *core.Profiles
	main           *mygo.Window
	tray           *mygo.Tray
	panel          *mygo.Window
	panelReady     bool
	panelPending   bool
	quitting       bool
	stopping       bool
	quitError      string
	startupReady   bool
	startupError   string
	trayError      string
	loginRead      func() bool
	loginWrite     func(bool) error
	clipboardWrite func(string) error
	dockShow       func()
	dockHide       func()
	displayMu      sync.Mutex
	display        string
	trayStop       chan struct{}
	trayWake       chan struct{}
	trayIcon       []byte
	appsMu         sync.Mutex
	apps           map[string]*App
	appsOrder      []string
}
type DesktopState struct {
	QuitError    string `json:"quitError"`
	StartupReady bool   `json:"startupReady"`
	StartupError string `json:"startupError"`
	TrayError    string `json:"trayError"`
}

func (d *Desktop) ClearQuitError() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.quitError = ""
}

func (d *Desktop) State() DesktopState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DesktopState{QuitError: d.quitError, StartupReady: d.startupReady, StartupError: d.startupError, TrayError: d.trayError}
}
func (d *Desktop) completeStartup(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err != nil {
		d.startupError = err.Error()
	}
	d.startupReady = true
}
func (d *Desktop) OpenAtLogin() bool {
	if d.loginRead != nil {
		return d.loginRead()
	}
	return mygo.App.OpenAtLogin()
}
func (d *Desktop) SetOpenAtLogin(enabled bool) error {
	if d.loginWrite != nil {
		return d.loginWrite(enabled)
	}
	return mygo.App.SetOpenAtLogin(enabled)
}
func (d *Desktop) RequestWiFiAccess() error {
	var err error
	mygo.RunOnMain(func() { err = network.RequestAccess() })
	return err
}
func (d *Desktop) OpenDashboard() {
	d.mu.Lock()
	if d.quitting {
		d.mu.Unlock()
		return
	}
	w := d.main
	panel := d.panel
	d.panelPending = false
	d.mu.Unlock()
	if panel != nil {
		panel.Hide()
	}
	if w == nil {
		mygo.RunOnMain(func() { w = d.openMain() })
	}
	if w != nil {
		w.Show()
		w.Restore()
		w.Focus()
	}
	d.syncDockVisibility()
}
func (d *Desktop) CopyTerminalProxy(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("代理端口无效")
	}
	return d.CopyText(fmt.Sprintf("export http_proxy=http://127.0.0.1:%d https_proxy=http://127.0.0.1:%d all_proxy=socks5://127.0.0.1:%d", port, port, port))
}

func (d *Desktop) CopyText(text string) error {
	if text == "" {
		return fmt.Errorf("没有可复制的内容")
	}
	var err error
	if d.clipboardWrite != nil {
		err = d.clipboardWrite(text)
	} else {
		err = mygo.Clipboard.Write(transfer.TextData(text))
	}
	if err != nil {
		return fmt.Errorf("复制失败：%w", err)
	}
	return nil
}
func (d *Desktop) Quit() { mygo.App.Quit() }

// openMain creates the dashboard, or rebuilds it where it was after its page was released.
func (d *Desktop) openMain() *mygo.Window {
	d.mu.Lock()
	if d.main != nil || d.quitting {
		w := d.main
		d.mu.Unlock()
		return w
	}
	bounds := d.mainBounds
	d.mu.Unlock()
	url := "/"
	if bounds != nil {
		url = "/?restore=1"
	}
	// TitleBarHiddenInset keeps traffic lights only: the sidebar brand is the
	// sole "Rocket Science" label, so the system title bar does not repeat it.
	w := mygo.NewWindow(mygo.WindowOptions{Title: "Rocket Science", Width: 1200, Height: 780, MinWidth: 760, MinHeight: 560, TitleBarStyle: mygo.TitleBarHiddenInset, URL: url})
	if bounds != nil {
		w.SetBounds(*bounds)
	}
	d.mu.Lock()
	d.main = w
	d.mu.Unlock()
	w.Page().OnPageTitleUpdated(func(e *mygo.TitleEvent) { e.PreventDefault() })
	w.OnClosed(func() {
		d.mu.Lock()
		if d.main == w {
			d.main = nil
			d.pageBusy = false
		}
		d.mu.Unlock()
		d.syncDockVisibility()
	})
	w.OnShow(func() { d.syncDockVisibility() })
	w.OnHide(func() { d.syncDockVisibility() })
	w.OnClose(func(e *mygo.CloseEvent) {
		d.mu.Lock()
		quitting := d.quitting
		hasTray := d.tray != nil
		d.mu.Unlock()
		if !quitting && hasTray {
			e.PreventDefault()
			w.Hide()
		}
	})
	return w
}

func (d *Desktop) ready() {
	d.openMain()
	d.watchIdlePages()
	d.syncDockVisibility()
	icon, err := trayAsset.ReadFile("assets/tray.png")
	if err != nil {
		log.Printf("tray icon: %v", err)
		return
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{ID: "rocket", Icon: icon, IconIsTemplate: true, ToolTip: "Rocket Science"})
	if err != nil {
		log.Printf("tray: %v", err)
		return
	}
	d.mu.Lock()
	d.tray = tray
	d.trayIcon = icon
	d.mu.Unlock()
	d.startTrayDisplay()
	// The flyout page is created on the first click.
	tray.OnClick(d.togglePanel)
}

func (d *Desktop) initPanel(tray *mygo.Tray) *mygo.Window {
	panel := mygo.NewFlyout(mygo.FlyoutOptions{Tray: tray, Placement: mygo.PlacementBottom, Width: 320, Height: 600, MinWidth: 320, MaxWidth: 320, MinHeight: 160, MaxHeight: 820, FitContent: true, Focusable: true, Hidden: true, Shadow: true, CornerRadius: 14, URL: "/?panel=tray"})
	d.mu.Lock()
	d.panel = panel
	d.panelReady = false
	d.mu.Unlock()
	panel.Page().OnDidNavigate(func(string) {
		d.mu.Lock()
		if d.panel == panel {
			d.panelReady = false
		}
		d.mu.Unlock()
	})
	panel.OnClosed(func() {
		d.mu.Lock()
		if d.panel == panel {
			d.panel = nil
			d.panelReady = false
			d.panelPending = false
		}
		d.mu.Unlock()
	})
	panel.OnClose(func(e *mygo.CloseEvent) {
		d.mu.Lock()
		quitting := d.quitting
		hasTray := d.tray != nil
		d.panelPending = false
		d.mu.Unlock()
		if !quitting && hasTray {
			e.PreventDefault()
			panel.Hide()
		}
	})
	return panel
}

func (d *Desktop) TrayReady() {
	d.mu.Lock()
	d.panelReady = true
	pending := d.panelPending
	d.panelPending = false
	panel := d.panel
	tray := d.tray
	quitting := d.quitting
	d.mu.Unlock()
	if pending && !quitting && tray != nil && panel != nil {
		panel.SetAnchor(mygo.Rectangle{})
		panel.Show()
	}
}

func (d *Desktop) togglePanel() {
	d.mu.Lock()
	panel, tray, quitting, ready, pending := d.panel, d.tray, d.quitting, d.panelReady, d.panelPending
	if quitting || tray == nil {
		d.mu.Unlock()
		return
	}
	if panel != nil && !ready {
		d.panelPending = !pending
		d.mu.Unlock()
		return
	}
	d.panelPending = false
	d.mu.Unlock()
	if panel == nil {
		d.mu.Lock()
		d.panelPending = true
		d.mu.Unlock()
		d.initPanel(tray)
		return
	}
	if panel.IsVisible() {
		panel.Hide()
		return
	}
	panel.SetAnchor(mygo.Rectangle{})
	panel.Show()
}

func (d *Desktop) ClosePanel() {
	d.mu.Lock()
	panel := d.panel
	d.panelPending = false
	d.mu.Unlock()
	if panel != nil {
		panel.Hide()
	}
}
func (d *Desktop) beforeQuit(e *mygo.QuitEvent) {
	d.mu.Lock()
	if d.quitting {
		d.mu.Unlock()
		return
	}
	e.PreventDefault()
	if d.stopping {
		d.mu.Unlock()
		return
	}
	d.stopping = true
	d.quitError = ""
	d.mu.Unlock()
	go func() {
		err := d.service.Stop()
		d.mu.Lock()
		d.stopping = false
		if err != nil {
			d.quitError = "无法恢复网络，未退出：" + err.Error()
			d.mu.Unlock()
			d.OpenDashboard()
			return
		}
		d.quitting = true
		d.mu.Unlock()
		mygo.App.Quit()
	}()
}
func (d *Desktop) destroy() {
	d.mu.Lock()
	tray := d.tray
	panel := d.panel
	stop := d.trayStop
	d.trayStop = nil
	idleStop := d.idleStop
	d.idleStop = nil
	d.tray = nil
	d.panel = nil
	d.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if idleStop != nil {
		close(idleStop)
	}
	if panel != nil {
		panel.Close()
	}
	if tray != nil {
		tray.Destroy()
	}
}
