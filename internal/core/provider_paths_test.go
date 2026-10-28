package core

import (
	"os"
	"strings"
	"testing"
)

func TestProviderPathTraversalReservedAndDuplicateRejected(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "relative path traversal",
			content: `
proxy-providers:
  p1:
    type: http
    url: https://example.com/sub.yaml
    path: ../../outside.yaml
rules:
  - MATCH,DIRECT
`,
			wantErr: "不能超出工作目录",
		},
		{
			name: "absolute path",
			content: `
rule-providers:
  r1:
    type: http
    behavior: domain
    url: https://example.com/rules.yaml
    path: /tmp/outside.yaml
rules:
  - MATCH,DIRECT
`,
			wantErr: "不能使用绝对路径",
		},
		{
			name: "reserved config.yaml overwrite",
			content: `
proxy-providers:
  p1:
    type: http
    url: https://example.com/sub.yaml
    path: ./config.yaml
rules:
  - MATCH,DIRECT
`,
			wantErr: "保留文件冲突",
		},
		{
			name: "reserved heartbeat overwrite",
			content: `
proxy-providers:
  p1:
    type: http
    url: https://example.com/sub.yaml
    path: heartbeat
rules:
  - MATCH,DIRECT
`,
			wantErr: "保留文件冲突",
		},
		{
			name: "duplicate provider cache path",
			content: `
proxy-providers:
  p1:
    type: http
    url: https://example.com/sub1.yaml
    path: ./providers/shared.yaml
rule-providers:
  r1:
    type: http
    behavior: domain
    url: https://example.com/sub2.yaml
    path: providers/shared.yaml
rules:
  - MATCH,DIRECT
`,
			wantErr: "相同的缓存路径",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runtimeConfig(tc.content, 7890, 9090, "secret", "rule")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("runtimeConfig(%s): got err=%v, want substring %q", tc.name, err, tc.wantErr)
			}
			if os.Getenv("ROCKETSCIENCE_MIHOMO") != "" {
				err = validateProfile(tc.content)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("validateProfile(%s): got err=%v, want substring %q", tc.name, err, tc.wantErr)
				}
			}
		})
	}

	safe := `
external-ui: ../../outside-ui
external-ui-url: https://example.com/ui.zip
proxy-providers:
  p1:
    type: http
    url: https://example.com/sub1.yaml
    path: ./providers/p1.yaml
  p2:
    type: http
    url: https://example.com/sub2.yaml
    path: ./providers/p2.yaml
rules:
  - MATCH,DIRECT
`
	out, err := runtimeConfig(safe, 7890, 9090, "secret", "rule")
	if err != nil {
		t.Fatalf("safe provider paths rejected: %v", err)
	}
	if strings.Contains(string(out), "external-ui") {
		t.Fatalf("expected external-ui fields to be stripped from runtime config:\n%s", out)
	}
}
