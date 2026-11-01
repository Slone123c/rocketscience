//go:build darwin

package network

// TraySpeeds draws the stacked ↑/↓ menu-bar readout as a 36 px tall template PNG.
func TraySpeeds(up, down string) ([]byte, bool) {
	if load() != nil {
		return nil, false
	}
	pointer := traySpeedsFn(up, down)
	if pointer == nil {
		return nil, false
	}
	var image struct {
		PNG []byte `json:"png"`
	}
	if err := decode(pointer, 1<<20, &image); err != nil || len(image.PNG) == 0 {
		return nil, false
	}
	return image.PNG, true
}
