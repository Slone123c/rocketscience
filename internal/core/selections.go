package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// selectionStore keeps each profile's Selector choices in selections.json, so a
// choice made while the core is stopped, or before the app quit, applies on the
// next start. It has its own lock and never calls Profiles or Service, so the
// core can save while holding Service.mu.
type selectionStore struct {
	mu   sync.Mutex
	path string
}

func (st *selectionStore) read() map[string]map[string]string {
	all := map[string]map[string]string{}
	data, err := os.ReadFile(st.path)
	if err != nil || json.Unmarshal(data, &all) != nil {
		return map[string]map[string]string{}
	}
	return all
}

func (st *selectionStore) write(all map[string]map[string]string) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o700); err != nil {
		return err
	}
	temp := st.path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, st.path)
}

func (st *selectionStore) load(profile string) map[string]string {
	if st == nil || profile == "" {
		return map[string]string{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	saved := map[string]string{}
	for group, node := range st.read()[profile] {
		saved[group] = node
	}
	return saved
}

// save replaces one profile's choices.
func (st *selectionStore) save(profile string, selected map[string]string) error {
	if st == nil || profile == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	all := st.read()
	copied := make(map[string]string, len(selected))
	for group, node := range selected {
		copied[group] = node
	}
	all[profile] = copied
	return st.write(all)
}

func (st *selectionStore) set(profile, group, node string) error {
	if st == nil || profile == "" {
		return errors.New("没有正在使用的配置")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	all := st.read()
	if all[profile] == nil {
		all[profile] = map[string]string{}
	}
	all[profile][group] = node
	return st.write(all)
}

// selectionsLocked returns the store next to profiles.json. Called with p.mu held.
func (p *Profiles) selectionsLocked() (*selectionStore, error) {
	if p.selections != nil {
		return p.selections, nil
	}
	dir, err := p.directory()
	if err != nil {
		return nil, err
	}
	p.selections = &selectionStore{path: filepath.Join(dir, "selections.json")}
	return p.selections, nil
}

// bindSelections points the core at a profile's saved choices. While running,
// the live choices are what that profile now uses, so they are saved; while
// stopped, the saved choices are loaded for the next start.
func (s *Service) bindSelections(store *selectionStore, profile string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectionStore, s.selectionProfile = store, profile
	if s.cmd != nil {
		s.persistSelectionsLocked()
		return
	}
	s.savedSelections = store.load(profile)
}

// persistSelectionsLocked is best effort: a failed write keeps the in-memory
// choices, which still apply until the app quits. Called with mu held.
func (s *Service) persistSelectionsLocked() {
	_ = s.selectionStore.save(s.selectionProfile, s.savedSelections)
}

// rememberSelectionLocked records a choice the core accepted. Called with mu held.
func (s *Service) rememberSelectionLocked(group, node string) {
	next := make(map[string]string, len(s.savedSelections)+1)
	for name, value := range s.savedSelections {
		next[name] = value
	}
	next[group] = node
	s.savedSelections = next
	s.persistSelectionsLocked()
}

// SelectPreview chooses the node a Selector group uses on the next start. It
// only works while the core is stopped; a running core takes Service.Select.
func (p *Profiles) SelectPreview(group, node string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil && p.service.Status().Running {
		return errors.New("内核运行中，请直接选择节点")
	}
	d, err := p.load()
	if err != nil {
		return err
	}
	content := ""
	for _, profile := range d.Profiles {
		if profile.ID == d.ActiveID {
			content = profile.Content
		}
	}
	if content == "" {
		return errors.New("没有正在使用的配置")
	}
	groups, err := previewProfileGroups(content)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if g.Name != group {
			continue
		}
		if g.Type != "Selector" || g.Name == unassignedGroupName {
			return errors.New("只有手动选择的策略组可以指定节点")
		}
		for _, member := range g.All {
			if member == node {
				store, err := p.selectionsLocked()
				if err != nil {
					return err
				}
				if err := store.set(d.ActiveID, group, node); err != nil {
					return err
				}
				if p.service != nil {
					p.service.bindSelections(store, d.ActiveID)
				}
				return nil
			}
		}
		return errors.New("节点不在该策略组中")
	}
	return errors.New("策略组不存在")
}
