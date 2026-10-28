package core

type ProfileSummary struct {
	ActiveID    string `json:"activeId"`
	MixedPort   int    `json:"mixedPort"`
	SystemProxy bool   `json:"systemProxy"`
}

// Summary omits YAML and subscription credentials from periodic tray snapshots.
func (p *Profiles) Summary() (ProfileSummary, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	return ProfileSummary{ActiveID: d.ActiveID, MixedPort: d.MixedPort, SystemProxy: d.SystemProxy}, err
}
