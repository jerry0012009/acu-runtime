//go:build !windows

package main

import (
	"context"
	"errors"
)

func upgradeOldGateway(context.Context, servingKernel) error {
	return errors.New("检测到旧版独立内核，请更新后重新连接")
}

func retireCompatibleGateway(context.Context, servingKernel) error { return nil }
