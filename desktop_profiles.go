package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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
