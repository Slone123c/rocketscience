package main

import (
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
)

type ImportedProfileFile struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (d *Desktop) PickProfileFile() (*ImportedProfileFile, error) {
	d.mu.Lock()
	parent := d.main
	d.mu.Unlock()
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "导入 Clash 配置", Filters: []mygo.FileFilter{{Name: "YAML 配置", Extensions: []string{"yaml", "yml"}}}})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	return readProfileFile(paths[0])
}

// ReadProfilePath imports a YAML path typed or pasted by hand, under the same limits as the file picker.
func (d *Desktop) ReadProfilePath(raw string) (*ImportedProfileFile, error) {
	path, err := normalizeProfilePath(raw)
	if err != nil {
		return nil, err
	}
	file, err := readProfileFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("找不到这个文件：" + path)
	}
	if errors.Is(err, os.ErrPermission) {
		return nil, errors.New("没有权限读取这个文件：" + path)
	}
	return file, err
}

// normalizeProfilePath accepts the forms a path gets copied in: plain, quoted,
// shell-escaped from a terminal drag, file:// from a browser, or starting with ~.
func normalizeProfilePath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", errors.New("请填写配置文件路径")
	}
	if strings.ContainsAny(path, "\r\n") {
		return "", errors.New("请只填写一个文件路径")
	}
	quoted := len(path) >= 2 && (path[0] == '"' || path[0] == '\'') && path[len(path)-1] == path[0]
	if quoted {
		path = path[1 : len(path)-1]
	} else if strings.HasPrefix(strings.ToLower(path), "file://") {
		parsed, err := url.Parse(path)
		if err != nil || (parsed.Host != "" && parsed.Host != "localhost") {
			return "", errors.New("只支持本机文件路径")
		}
		path = parsed.Path
		if runtime.GOOS == "windows" {
			if len(path) >= 3 && path[0] == '/' && ((path[1] >= 'a' && path[1] <= 'z') || (path[1] >= 'A' && path[1] <= 'Z')) && path[2] == ':' {
				path = path[1:]
			}
			path = filepath.FromSlash(path)
		}
	} else if runtime.GOOS != "windows" {
		path = unescapeShellPath(path)
	}
	if path == "~" || strings.HasPrefix(path, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(path, `~\`)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	if runtime.GOOS == "windows" && (strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//")) {
		return "", errors.New("只支持本机文件路径")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("请填写完整路径，例如 ~/Downloads/config.yaml")
	}
	return filepath.Clean(path), nil
}

func unescapeShellPath(path string) string {
	var out strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+1 < len(path) {
			i++
		}
		out.WriteByte(path[i])
	}
	return out.String()
}

func readProfileFile(path string) (*ImportedProfileFile, error) {
	ext := filepath.Ext(path)
	if strings.ToLower(ext) != ".yaml" && strings.ToLower(ext) != ".yml" {
		return nil, errors.New("请选择 .yaml 或 .yml 配置文件")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("配置必须是普通文件")
	}
	if info.Size() > 10*1024*1024 {
		return nil, errors.New("配置文件不能超过 10 MB")
	}
	data, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 10*1024*1024 {
		return nil, errors.New("配置文件不能超过 10 MB")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("配置文件必须使用 UTF-8 编码")
	}
	return &ImportedProfileFile{Path: path, Name: strings.TrimSuffix(filepath.Base(path), ext), Content: string(data)}, nil
}
