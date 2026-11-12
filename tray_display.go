package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"time"
	"unicode/utf8"

	"image/draw"
	"strings"

	"rocketscience/internal/core"
	"rocketscience/internal/network"
)

type traySurface interface {
	SetIcon([]byte, bool) error
	SetTitle(string)
}
type trayFrame struct {
	icon  bool
	title string
	// up and down feed the stacked image readout; title is the one-line fallback.
	up, down string
	// drawn records that the speeds went out as an image rather than as the title.
	drawn bool
}

// drawSpeeds renders the stacked ↑/↓ readout; nil, or a failed draw, falls back to the title.
var drawSpeeds = network.TraySpeeds

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
	return trayFrame{icon: display != "speed", title: "↑" + up + " ↓" + down, up: strings.TrimLeft(up, "\u2007"), down: strings.TrimLeft(down, "\u2007")}
}

// besideIcon puts the 32 px rocket, centred in the 36 px tall canvas, left of the speeds.
func besideIcon(icon, speeds []byte) ([]byte, error) {
	rocket, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, err
	}
	text, err := png.Decode(bytes.NewReader(speeds))
	if err != nil {
		return nil, err
	}
	const gap = 5
	r, t := rocket.Bounds(), text.Bounds()
	height := max(r.Dy(), t.Dy())
	canvas := image.NewRGBA(image.Rect(0, 0, r.Dx()+gap+t.Dx(), height))
	draw.Draw(canvas, image.Rect(0, (height-r.Dy())/2, r.Dx(), (height-r.Dy())/2+r.Dy()), rocket, r.Min, draw.Src)
	draw.Draw(canvas, image.Rect(r.Dx()+gap, (height-t.Dy())/2, r.Dx()+gap+t.Dx(), (height-t.Dy())/2+t.Dy()), text, t.Min, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func transparentTrayIcon() []byte {
	var output bytes.Buffer
	_ = png.Encode(&output, image.NewRGBA(image.Rect(0, 0, 1, 18)))
	return output.Bytes()
}

// applyTrayFrame reports whether the speeds went out as an image, so the next
// frame knows whether a title is still showing.
func applyTrayFrame(tray traySurface, frame, previous trayFrame, first bool, icon, empty []byte) (bool, error) {
	if frame.title != "" && drawSpeeds != nil {
		if !first && previous.drawn && frame.icon == previous.icon && frame.up == previous.up && frame.down == previous.down {
			return true, nil
		}
		if speeds, ok := drawSpeeds(frame.up, frame.down); ok {
			img := speeds
			if frame.icon {
				var err error
				if img, err = besideIcon(icon, speeds); err != nil {
					return false, err
				}
			}
			if err := tray.SetIcon(img, true); err != nil {
				return false, err
			}
			if first || !previous.drawn {
				tray.SetTitle("")
			}
			return true, nil
		}
	}
	// A frame after an image readout always resets the icon it replaced.
	if first || previous.drawn || frame.icon != previous.icon {
		img := empty
		if frame.icon {
			img = icon
		}
		if err := tray.SetIcon(img, true); err != nil {
			return false, err
		}
	}
	if first || previous.drawn || frame.title != previous.title {
		tray.SetTitle(frame.title)
	}
	return false, nil
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
// trayTickInterval is 1s when speeds are shown; icon-only only needs occasional
// refresh because applyTrayFrame already skips unchanged frames.
func trayTickInterval(display string) time.Duration {
	if display == "icon" {
		return 5 * time.Second
	}
	return time.Second
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
		interval := trayTickInterval(mode)
		tick := time.NewTicker(interval)
		defer tick.Stop()
		previous := trayFrame{}
		first := true
		empty := transparentTrayIcon()
		syncTicker := func(display string) {
			next := trayTickInterval(display)
			if next == interval {
				return
			}
			tick.Stop()
			tick = time.NewTicker(next)
			interval = next
		}
		render := func() {
			d.mu.Lock()
			display := d.display
			d.mu.Unlock()
			syncTicker(display)
			running := false
			data := core.Telemetry{}
			if d.service != nil {
				running = d.service.Status().Running
				if display != "icon" {
					data = d.service.LiveTelemetry()
				}
			}
			frame := makeTrayFrame(display, running, data)
			if !first && frame.icon == previous.icon && frame.title == previous.title && frame.up == previous.up && frame.down == previous.down {
				return
			}
			drawn, err := applyTrayFrame(tray, frame, previous, first, icon, empty)
			frame.drawn = drawn
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
