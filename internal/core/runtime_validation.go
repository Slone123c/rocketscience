package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func validateRuntimeWithContext(ctx context.Context, bin, dir, path string) error {
	output, err := exec.CommandContext(ctx, bin, "-t", "-d", dir, "-f", path).CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("mihomo 校验未完成，请检查配置中的外部资源和网络连接：%w", ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("mihomo 校验失败：%s", strings.TrimSpace(string(output)))
	}
	return nil
}
