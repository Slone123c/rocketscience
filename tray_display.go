package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"time"
	"unicode/utf8"

	"rocketscience/internal/core"
)

type traySurface interface {
	SetIcon([]byte, bool) error
	SetTitle(string)
}
type trayFrame struct {
	icon  bool
	title string
}

func padSpeed(text string) string {
	for utf8.RuneCountInString(text) < 4 {
		text = "\u2007" + text
	}
	return text
}

func speedNumber(rate float64, index int) string {
	if index > 0 {
		if number := fmt.Sprintf("%.1f", rate); len(number) <= 3 {
			return number
		}
	}
	return fmt.Sprintf("%.0f", rate)
}

// speedText matches TrayIndicator.shortSpeed: at most four characters, so the
// menu-bar title keeps a stable width. The unit is chosen after rounding, so
// 999.6K becomes 1.0M rather than 1000K, and the unit divides by 1024.
func speedText(value int64) string {
	if value < 0 {
		value = 0
	}
	units := []string{"B", "K", "M", "G"}
	rate := float64(value)
	index := 0
	number := speedNumber(rate, index)
	for len(number) > 3 && index < len(units)-1 {
		rate /= 1024
		index++
		number = speedNumber(rate, index)
	}
	return padSpeed(number + units[index])
}
func makeTrayFrame(display string, running bool, data core.Telemetry) trayFrame {
	if display == "icon" || (display == "both" && !running) {
		return trayFrame{icon: true}
	}
	up, down := padSpeed("—"), padSpeed("—")
	if running && data.Connected {
		up, down = speedText(data.Up), speedText(data.Down)
	}
	return trayFrame{icon: display != "speed", title: "↑" + up + " ↓" + down}
}
func transparentTrayIcon() []byte {
	var output bytes.Buffer
	_ = png.Encode(&output, image.NewRGBA(image.Rect(0, 0, 1, 18)))
	return output.Bytes()
}
func applyTrayFrame(tray traySurface, frame, previous trayFrame, first bool, icon, empty []byte) error {
	if first || frame.icon != previous.icon {
		img := empty
		if frame.icon {
			img = icon
		}
		if err := tray.SetIcon(img, true); err != nil {
			return err
		}
	}
	if first || frame.title != previous.title {
		tray.SetTitle(frame.title)
	}
	return nil
}
func (d *Desktop) TrayDisplay() (string, error) {
	if d.profiles == nil {
		return "", errors.New("设置不可用")
	}
	return d.profiles.TrayDisplay()
}
func (d *Desktop) SetTrayDisplay(value string) error {
	d.displayMu.Lock()
	defer d.displayMu.Unlock()
	if d.profiles == nil {
		return errors.New("设置不可用")
	}
	if err := d.profiles.SetTrayDisplay(value); err != nil {
		return err
	}
	d.mu.Lock()
	d.display = value
	wake := d.trayWake
	d.mu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	return nil
}
func (d *Desktop) startTrayDisplay() {
	mode := "both"
	if d.profiles != nil {
		if value, err := d.profiles.TrayDisplay(); err == nil {
			mode = value
		}
	}
	d.mu.Lock()
	d.display = mode
	d.trayStop = make(chan struct{})
	d.trayWake = make(chan struct{}, 1)
	stop, wake := d.trayStop, d.trayWake
	tray, icon := d.tray, d.trayIcon
	d.mu.Unlock()
	if tray == nil {
		return
	}
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		previous := trayFrame{}
		first := true
		empty := transparentTrayIcon()
		render := func() {
			d.mu.Lock()
			display := d.display
			d.mu.Unlock()
			running := false
			data := core.Telemetry{}
			if d.service != nil {
				running = d.service.Status().Running
				if display != "icon" {
					data = d.service.LiveTelemetry()
				}
			}
			frame := makeTrayFrame(display, running, data)
			err := applyTrayFrame(tray, frame, previous, first, icon, empty)
			d.mu.Lock()
			if err != nil {
				d.trayError = "菜单栏显示更新失败：" + err.Error()
			} else {
				d.trayError = ""
			}
			d.mu.Unlock()
			if err == nil {
				previous = frame
				first = false
			}
		}
		render()
		for {
			select {
			case <-stop:
				return
			case <-wake:
				render()
			case <-tick.C:
				render()
			}
		}
	}()
}
