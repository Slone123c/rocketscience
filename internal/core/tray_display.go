package core

import "errors"

func validTrayDisplay(value string) bool {
	return value == "icon" || value == "speed" || value == "both"
}
func (p *Profiles) TrayDisplay() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return "", err
	}
	if !validTrayDisplay(d.TrayDisplay) {
		return "both", nil
	}
	return d.TrayDisplay, nil
}
func (p *Profiles) SetTrayDisplay(value string) error {
	if !validTrayDisplay(value) {
		return errors.New("菜单栏显示方式无效")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return err
	}
	d.TrayDisplay = value
	return p.write(d)
}
