package core

import (
	"errors"
	"time"
)

// StartActive uses the latest saved content, even if an update completed just before launch.
func (p *Profiles) StartActive() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service == nil {
		return errors.New("内核服务不可用")
	}
	d, err := p.load()
	if err != nil {
		return err
	}
	return p.startActiveLocked(d)
}

func (p *Profiles) startActiveLocked(d profileData) error {
	for _, v := range d.Profiles {
		if v.ID == d.ActiveID {
			merged, _, _, _, err := mergeOverrides(v.Content, d.Overrides)
			if err != nil {
				return err
			}
			if err = p.service.start(merged, d.MixedPort, d.ProxyBypass, lanValue(d.LAN), coreOptionsValue(d.CoreOptions)); err != nil {
				return err
			}
			p.draft = v.Content
			return nil
		}
	}
	return errors.New("没有正在使用的配置")
}

// Update fetches a subscription and changes its saved and running configuration together.
func (p *Profiles) Update(id string) (ProfileState, error) {
	return p.updateSubscription(id, false)
}

func (p *Profiles) updateSubscription(id string, onlyIfDue bool) (ProfileState, error) {
	p.updateMu.Lock()
	defer p.updateMu.Unlock()
	p.mu.Lock()
	d, err := p.load()
	if err != nil {
		p.mu.Unlock()
		return ProfileState{}, err
	}
	var captured Profile
	found := false
	for _, v := range d.Profiles {
		if v.ID == id {
			captured = v
			found = true
			break
		}
	}
	if found && onlyIfDue && !subscriptionDue(captured, time.Now()) {
		state := ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}
		p.mu.Unlock()
		return state, nil
	}
	p.mu.Unlock()
	if !found {
		return ProfileState{}, errors.New("配置不存在")
	}
	if captured.Source != "url" {
		return ProfileState{}, errors.New("该配置不是订阅")
	}
	if err = validateMeta(captured); err != nil {
		return ProfileState{}, err
	}
	content, usage, fetchErr := downloadProfile(captured.URL)
	if fetchErr == nil {
		fetchErr = validateProfile(content)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err = p.load()
	if err != nil {
		return ProfileState{}, err
	}
	index := -1
	for i, v := range d.Profiles {
		if v.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return ProfileState{}, errors.New("配置已删除")
	}
	current := d.Profiles[index]
	if current.URL != captured.URL || current.Source != captured.Source || current.Content != captured.Content || current.Interval != captured.Interval {
		return ProfileState{}, errors.New("配置在更新期间发生变化，请重新更新")
	}
	current.LastAttempt = time.Now().UTC()
	if fetchErr == nil {
		next := current
		next.Content, next.Usage = content, usage
		next.UpdatedAt, next.LastError = current.LastAttempt, ""
		d.Profiles[index] = next
		if d.ActiveID == id {
			fetchErr = p.commitProfile(d, content)
		} else {
			fetchErr = p.write(d)
		}
	}
	if fetchErr != nil {
		current.LastError = fetchErr.Error()
		d.Profiles[index] = current
		if err = p.write(d); err != nil {
			return ProfileState{}, errors.Join(fetchErr, err)
		}
		return ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}, fetchErr
	}
	return ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}, nil
}

func subscriptionDue(v Profile, now time.Time) bool {
	if v.Source != "url" || v.Interval <= 0 {
		return false
	}
	base := v.LastAttempt
	if base.IsZero() {
		base = v.UpdatedAt
	}
	return base.IsZero() || !now.Before(base.Add(time.Duration(v.Interval)*time.Hour))
}

// RunUpdates checks subscriptions while the app is open; stopping the channel ends scheduling.
func RunUpdates(p *Profiles, stop <-chan struct{}) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		state, err := p.State()
		if err == nil {
			for _, v := range state.Profiles {
				select {
				case <-stop:
					return
				default:
				}
				if subscriptionDue(v, time.Now()) {
					_, _ = p.updateSubscription(v.ID, true)
				}
			}
		}
		select {
		case <-stop:
			return
		case <-tick.C:
		}
	}
}
