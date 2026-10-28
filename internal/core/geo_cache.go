package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// Only files owned by mihomo's GEO database subsystem are copied. Configuration,
// subscriptions, and unrelated provider files never enter this cache.
var geoNames = map[string]bool{
	"country.mmdb": true, "geoip.db": true, "geoip.metadb": true,
	"geoip.dat": true, "geosite.dat": true, "asn.mmdb": true,
	"cache.db": true,
}

type geoCache struct {
	mu  sync.Mutex
	dir string
}

func defaultGeoCacheDir() (string, error) {
	if configured := os.Getenv("ROCKETSCIENCE_DATA_DIR"); configured != "" {
		return filepath.Join(configured, "geo"), nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "RocketScience", "geo"), nil
}
func (c *geoCache) path() (string, error) {
	if c.dir != "" {
		return c.dir, nil
	}
	return defaultGeoCacheDir()
}
func isGeoName(name string) bool { return geoNames[strings.ToLower(name)] }

// copyGeoFiles atomically replaces complete destination files and never follows
// symlinks or removes the last known good cache when a copy fails.
func copyGeoFiles(source, destination string) error {
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !isGeoName(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		sourcePath := filepath.Join(source, entry.Name())
		fd, err := syscall.Open(sourcePath, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return fmt.Errorf("读取 GEO 数据失败：%w", err)
		}
		input := os.NewFile(uintptr(fd), sourcePath)
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() {
			input.Close()
			if err != nil {
				return err
			}
			return errors.New("GEO 数据不是普通文件")
		}
		if err = os.MkdirAll(destination, 0700); err != nil {
			input.Close()
			return err
		}
		output, err := os.CreateTemp(destination, ".geo-copy-*")
		if err != nil {
			input.Close()
			return err
		}
		tmp := output.Name()
		if err = output.Chmod(0600); err == nil {
			_, err = io.Copy(output, input)
		}
		if closeErr := input.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = output.Sync()
		}
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmp, filepath.Join(destination, entry.Name()))
		}
		_ = os.Remove(tmp)
		if err != nil {
			return fmt.Errorf("保存 GEO 数据失败：%w", err)
		}
	}
	return nil
}
func (c *geoCache) seed(runtimeDir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	dir, err := c.path()
	if err != nil {
		return err
	}
	return copyGeoFiles(dir, runtimeDir)
}
func (c *geoCache) save(runtimeDir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	dir, err := c.path()
	if err != nil {
		return err
	}
	return copyGeoFiles(runtimeDir, dir)
}
