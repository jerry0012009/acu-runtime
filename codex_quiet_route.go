package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type codexFieldChange struct{ Field, Value string }

// Upstream's live route watcher ends ChatGPT's hosted app-server on Connect
// and Disconnect even with autoRestart off. Write while our own idle watcher
// is stopped; a new watcher starts with the complete route as its baseline.
func (k *kernelState) changeCodexQuietly(ctx context.Context, changes []codexFieldChange, disconnect bool) (result map[string]json.RawMessage, resultErr error) {
	return k.changeCodexQuietlyWith(ctx, changes, disconnect, nil)
}

func (k *kernelState) changeCodexQuietlyWith(ctx context.Context, changes []codexFieldChange, disconnect bool, mutation func(string) error) (result map[string]json.RawMessage, resultErr error) {
	k.routeMu.Lock()
	defer k.routeMu.Unlock()
	k.mu.Lock()
	command, executable, starting := k.process, k.exe, k.starting
	k.mu.Unlock()
	if starting || command == nil || command.Process == nil {
		return nil, errors.New("当前内核不由 Runtime 独立管理，未更改 Codex 配置")
	}
	info, err := readServingKernel(ctx)
	if err != nil || uint32(command.Process.Pid) != info.PID {
		return nil, errors.New("无法核实 Runtime 内核进程，未更改 Codex 配置")
	}
	k.mu.Lock()
	if k.starting || k.process != command {
		k.mu.Unlock()
		return nil, errors.New("内核状态已变化，请重试")
	}
	k.routePhase = "等待现有请求结束"
	k.mu.Unlock()
	defer func() { k.mu.Lock(); k.routePhase = ""; k.mu.Unlock() }()
	err = waitGatewayIdleForChange(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, errors.New("本机网关仍有请求或工具在运行。请在对应应用停止卡住的请求或等它完成后重试；未更改 Codex 配置")
	}
	if err != nil {
		return nil, errors.New("无法确认本机网关已空闲，请重试；未更改 Codex 配置")
	}
	previous, err := k.readState(ctx)
	if err != nil {
		return nil, errors.New("无法读取原 Codex 设置，未更改配置")
	}
	probeModel := ""
	if !disconnect {
		var agents []setupAgent
		json.Unmarshal(previous["agents"], &agents)
		for _, agent := range agents {
			if agent.ID == "codex" {
				for _, field := range agent.Fields {
					if field.Key == "model" {
						if agent.Wired {
							probeModel = field.Value
						}
					}
				}
			}
		}
	}
	model := probeModel
	for _, change := range changes {
		if change.Field == "model" {
			model = change.Value
		}
	}
	snapshot, err := snapshotCodexRoute(executable)
	if err != nil {
		return nil, errors.New("无法备份 Codex 配置，未更改配置")
	}
	authPath := filepath.Join(codexConfigDirectory(), "auth.json")
	authBefore, authErr := os.ReadFile(authPath)
	if authErr != nil && !os.IsNotExist(authErr) {
		return nil, errors.New("无法读取 Codex 登录配置，请重试")
	}
	k.mu.Lock()
	k.routePhase = "正在写入并核验 Codex 配置"
	k.mu.Unlock()
	if err = command.Process.Kill(); err != nil {
		return nil, errors.New("无法暂停内核路由观察，未更改 Codex 配置")
	}
	recovery, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	restored := false
	defer func() {
		rolledBack := false
		if resultErr != nil && !restored {
			if e := snapshot.restore(); e != nil {
				resultErr = errors.New("配置验证失败，自动恢复未完成；Codex 接入未通过验收")
			} else {
				rolledBack = true
			}
		}
		// Wait is already owned by start's goroutine; never call Cmd.Wait twice.
		for {
			k.mu.Lock()
			gone := k.process == nil
			k.mu.Unlock()
			if gone {
				break
			}
			select {
			case <-recovery.Done():
				resultErr = errors.New("内核退出超时，请重新打开 Runtime")
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
		k.mu.Lock()
		stopping := k.stopRequested
		k.mu.Unlock()
		if stopping {
			return
		}
		if e := k.start(recovery); e != nil {
			// Native validation alone is not a commit: the new gateway must
			// also start. Leave the old route on disk if recovery fails.
			k.close()
			exitDeadline := time.NewTimer(5 * time.Second)
			defer exitDeadline.Stop()
			for {
				k.mu.Lock()
				gone := k.process == nil
				k.mu.Unlock()
				if gone {
					break
				}
				select {
				case <-exitDeadline.C:
					resultErr = errors.New("无法确认内核已暂停，原配置备份仍保留在本次操作中；请重新打开 Runtime")
					return
				case <-time.After(20 * time.Millisecond):
				}
			}
			if e := snapshot.restore(); e != nil {
				resultErr = errors.New("内核恢复失败且无法完整还原配置，请重新打开 Runtime 修复")
				return
			}
			resultErr = errors.New("内核尚未恢复连接，已恢复原配置；请重新打开 Runtime")
			return
		}
		if rolledBack {
			resultErr = codexRollbackCompleted(resultErr)
		}
		if resultErr == nil {
			// Signed-in Codex reads Magpie's live /models endpoint. Validate
			// only after the replacement gateway serves the complete route.
			validate := k.codexValidate
			if validate == nil {
				validate = validateInstalledCodex
			}
			if model != "" {
				resultErr = validate(ctx, model)
				if errors.Is(resultErr, errDesktopProbeUnavailable) {
					resultErr = k.verifyDesktopCodexWithoutProbe(ctx, model)
				}
			}
			if resultErr == nil {
				resultErr = verifyCodexAuthUnchanged(authPath, authBefore, authErr)
			}
			if resultErr == nil {
				result, resultErr = k.readState(recovery)
			}
			if resultErr == nil && (len(changes) > 0 || disconnect || mutation != nil && probeModel != "") {
				resultErr = verifyCodexRouteState(result, changes, disconnect)
			}
			if resultErr != nil {
				k.close()
				for {
					k.mu.Lock()
					gone := k.process == nil
					k.mu.Unlock()
					if gone {
						break
					}
					select {
					case <-recovery.Done():
						return
					case <-time.After(20 * time.Millisecond):
					}
				}
				if e := snapshot.restore(); e != nil {
					resultErr = errors.New("配置核验失败，自动恢复未完成")
					return
				}
				if e := k.start(recovery); e != nil {
					resultErr = errors.New("配置已恢复，内核恢复连接失败，请重新打开 Runtime")
					return
				}
				resultErr = codexRollbackCompleted(resultErr)
			}
		}
	}()
	for {
		k.mu.Lock()
		gone := k.process == nil
		k.mu.Unlock()
		if gone {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	run := func(args ...string) error {
		c := exec.CommandContext(ctx, executable, args...)
		hideProcess(c)
		cleanup, err := startOwnedCommand(c)
		if err != nil {
			return err
		}
		defer cleanup()
		return c.Wait()
	}
	if mutation != nil {
		err = mutation(executable)
	} else if disconnect {
		err = run("codex", "default")
	} else {
		// Let the native default adapt to the account, and retain explicit
		// login choices. Selecting a model must not force API-only mode.
		for _, change := range changes {
			value := change.Value
			if value == "" {
				value = "default"
			}
			if err = run("codex", change.Field, value); err != nil {
				break
			}
		}
	}
	if err != nil {
		// A CLI may exit unsuccessfully after persisting an idempotent change.
		// Only explicit field/disconnect operations have a complete readback
		// contract here; arbitrary catalog mutations must still roll back.
		if mutation != nil || ctx.Err() != nil || len(changes) == 0 && !disconnect {
			return nil, errors.New("Codex 本次修改未完成")
		}
		if readErr := nativeCatalogManagement(ctx, executable, func(control *kernelState) error {
			return verifyCodexCommandOutcome(ctx, control, changes, disconnect)
		}); readErr != nil {
			return nil, errors.New("Codex 本次修改未完成")
		}
	}
	if !disconnect {
		if err = brandCodexProvider(); err != nil {
			return nil, errors.New("无法保存 Codex 显示名称，请重试")
		}
		if err = configureCodexGeminiTools(); err != nil {
			return nil, errors.New("无法保存 Gemini 工具配置")
		}
	}
	if err = verifyCodexAuthUnchanged(authPath, authBefore, authErr); err != nil {
		return nil, err
	}
	restored = true
	return nil, nil
}

// A registered Store app need not allow its embedded CLI to run on its own.
// Reuse the running kernel's actual route and visible Codex catalog in that
// case only. A response from a running Codex is always authoritative.
func (k *kernelState) verifyDesktopCodexWithoutProbe(ctx context.Context, model string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if model == codexHydrogenModel {
		return errors.New("无法核验 Gemini Hydrogen 的 Code Mode 组件，请在 Runtime 中更新 ChatGPT，或选择其他模型")
	}
	if err := verifyDesktopCodexFiles(model); err != nil {
		return err
	}
	state, err := k.readState(ctx)
	if err != nil {
		return err
	}
	if err := verifyCodexRouteState(state, []codexFieldChange{{"model", model}}, false); err != nil {
		return err
	}
	var catalog agentCatalog
	if err := k.exchange(ctx, "GET", "/api/agent-models/codex", nil, &catalog); err != nil {
		return err
	}
	for _, entry := range catalog.Models {
		if entry.ID == model && !entry.Hidden && !entry.Kept {
			return ctx.Err()
		}
	}
	return errors.New("内核未在 Codex 的可用模型目录中找到所选模型")
}

func codexRollbackCompleted(cause error) error {
	return errors.New(cause.Error() + "；已恢复此前配置，本次修改未生效")
}

func verifyCodexCommandOutcome(ctx context.Context, control *kernelState, changes []codexFieldChange, disconnect bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(changes) == 0 && !disconnect {
		return errors.New("缺少可核验的 Codex 修改目标")
	}
	state, err := control.readState(ctx)
	if err != nil {
		return err
	}
	return verifyCodexRouteState(state, changes, disconnect)
}

func verifyCodexRouteState(state map[string]json.RawMessage, changes []codexFieldChange, disconnect bool) error {
	var agents []setupAgent
	if json.Unmarshal(state["agents"], &agents) != nil {
		return errors.New("无法核对 Codex 状态")
	}
	for _, a := range agents {
		if a.ID != "codex" {
			continue
		}
		if disconnect {
			if a.Wired {
				return errors.New("Codex 未完成断开")
			}
			return nil
		}
		if !a.Wired {
			return errors.New("Codex 未接入")
		}
		if a.needsRepair() {
			return errors.New("Codex 接入仍存在配置或连接异常，请检查登录模式、代理及高级配置")
		}
		for _, change := range changes {
			found := false
			for _, field := range a.Fields {
				if field.Key == change.Field {
					found = field.Value == change.Value || change.Value == "default" && field.Value == ""
					break
				}
			}
			if !found {
				return errors.New("Codex 配置读取结果不一致")
			}
		}
		return nil
	}
	return errors.New("无法找到 Codex 状态")
}
