//go:build !windows

package main

import (
	"context"
	"errors"
)

func validateInstalledCodex(context.Context, string) error {
	return errors.New("当前平台尚未启用 Codex 安全路由切换")
}
