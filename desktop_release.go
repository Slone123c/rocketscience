package main

import (
	"runtime/debug"
	"time"

	"github.com/egoist/mygo"
)

const (
	// A hidden page keeps its WebKit processes and graphics buffers, so it is
	// released after this long and rebuilt when it is next opened.
	pageIdleRelease   = 60 * time.Second
	pageReleaseChecks = 10 * time.Second
)

// pageIdle measures how long a window has stayed hidden.
type pageIdle struct{ hiddenSince time.Time }

func (p *pageIdle) expired(hidden, keep bool, now time.Time) bool {
	if !hidden || keep {
		p.hiddenSince = time.Time{}
		return false
	}
	if p.hiddenSince.IsZero() {
		p.hiddenSince = now
		return false
	}
	return now.Sub(p.hiddenSince) >= pageIdleRelease
}

// SetPageBusy keeps the dashboard page alive while it holds work that a rebuild would lose.
func (d *Desktop) SetPageBusy(busy bool) {
	d.mu.Lock()
	d.pageBusy = busy
	d.mu.Unlock()
}

func (d *Desktop) watchIdlePages() {
	d.mu.Lock()
	stop := make(chan struct{})
	d.idleStop = stop
	d.mu.Unlock()
	go func() {
		ticker := time.NewTicker(pageReleaseChecks)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mygo.RunOnMain(d.releaseIdlePages)
			}
		}
	}()
}

// releaseIdlePages runs on the main thread. The core, the tray display and all
// saved state live in this process, so only the pages go away.
func (d *Desktop) releaseIdlePages() {
	d.mu.Lock()
	quitting, stopping, main, panel, busy, pending := d.quitting, d.stopping, d.main, d.panel, d.pageBusy, d.panelPending
	d.mu.Unlock()
	if quitting || stopping {
		return
	}
	now, released := time.Now(), false
	// A minimized window stays in the Dock, so it is not treated as hidden.
	if main != nil && d.mainIdle.expired(!main.IsVisible() && !main.IsMinimized(), busy, now) {
		bounds := main.Bounds()
		d.mu.Lock()
		d.mainBounds = &bounds
		d.mu.Unlock()
		main.Destroy()
		released = true
	}
	if panel != nil && d.panelIdle.expired(!panel.IsVisible(), pending, now) {
		panel.Destroy()
		released = true
	}
	if released {
		go debug.FreeOSMemory()
	}
}
