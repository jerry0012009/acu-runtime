//go:build !windows && !darwin

package main

import (
	"context"
	"errors"
)

func installWorkBuddy(context.Context, string) error {
	return errors.New("此平台暂不支持桌面自动安装")
}
func openChatGPTInstaller(context.Context) error {
	return errors.New("此平台暂不支持桌面安装")
}
func openInstalledAgent(context.Context, string) error { return errors.New("请手动打开应用") }

func workBuddyInstalled() bool { return false }
