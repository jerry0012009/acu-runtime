//go:build !windows && !darwin

package main

import (
	"context"
	"errors"
)

func openTapnowDirectory(context.Context, string) error {
	return errors.New("请手动打开所示插件目录")
}
