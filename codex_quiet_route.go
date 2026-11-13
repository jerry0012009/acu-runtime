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
	k.starting = true
	k.mu.Unlock()
	defer func() { k.mu.Lock(); k.starting = false; k.mu.Unlock() }()
	if err = waitGatewayIdle(ctx); err != nil {
		return nil, errors.New("当前请求尚未完成，未更改 Codex 配置")
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
						probeModel = field.Value
					}
				}
			}
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
	if err = command.Process.Kill(); err != nil {
		return nil, errors.New("无法暂停内核路由观察，未更改 Codex 配置")
	}
	recovery, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	restored := false
	defer func() {
		if resultErr != nil && !restored {
			if e := snapshot.restore(); e != nil {
				resultErr = errors.New("配置验证失败，自动恢复未完成；Codex 接入未通过验收")
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
		if resultErr == nil {
			result, resultErr = k.readState(recovery)
			if resultErr == nil {
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
				resultErr = errors.New("未确认 Codex 配置生效，已恢复原配置")
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
	if disconnect {
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
		return nil, errors.New("Codex 配置未完成，正在恢复原配置")
	}
	if !disconnect {
		if err = brandCodexProvider(); err != nil {
			return nil, errors.New("无法保存 Codex 显示名称，请重试")
		}
	}
	model := probeModel
	for _, c := range changes {
		if c.Field == "model" {
			model = c.Value
		}
	}
	validate := k.codexValidate
	if validate == nil {
		validate = validateInstalledCodex
	}
	if err = validate(ctx, model); err != nil {
		return nil, err
	}
	if err = verifyCodexAuthUnchanged(authPath, authBefore, authErr); err != nil {
		return nil, err
	}
	restored = true
	return nil, nil
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
