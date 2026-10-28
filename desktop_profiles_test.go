package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeProfileFileImport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "公司配置.YAML")
	content := "rules:\n  - MATCH,DIRECT\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := readProfileFile(path)
	if err != nil || file.Path != path || file.Name != "公司配置" || file.Content != content {
		t.Fatalf("import changed source: %+v %v", file, err)
	}
	for _, test := range []struct {
		name string
		data []byte
	}{{"binary.yaml", []byte{0xff}}, {"not-yaml.txt", []byte(content)}} {
		path := filepath.Join(dir, test.name)
		if err := os.WriteFile(path, test.data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readProfileFile(path); err == nil {
			t.Fatalf("invalid file accepted: %s", test.name)
		}
	}
	folder := filepath.Join(dir, "folder.yaml")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := readProfileFile(folder); err == nil {
		t.Fatal("directory accepted as YAML")
	}
	large := filepath.Join(dir, "large.yaml")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(10*1024*1024 + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := readProfileFile(large); err == nil {
		t.Fatal("oversized import accepted")
	}
}
