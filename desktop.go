//go:build desktop

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const desktopEnabled = true

func desktopIcon() []byte {
	canvas := image.NewRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 32, G: 33, B: 32, A: 255}}, image.Point{}, draw.Src)
	letter := []string{"00100", "01010", "10001", "11111", "10001", "10001", "10001"}
	for row, cells := range letter {
		for column, pixel := range cells {
			if pixel == '1' {
				draw.Draw(canvas, image.Rect(17+column*6, 11+row*6, 23+column*6, 17+row*6), &image.Uniform{C: color.RGBA{R: 235, G: 141, B: 34, A: 255}}, image.Point{}, draw.Src)
			}
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		panic(err)
	}
	return output.Bytes()
}

func runDesktop(ctx context.Context, host *server, origin string, inspectPort int, quitRequests <-chan struct{}, shutdown func()) error {
	if inspectPort < 0 || inspectPort > 65535 {
		return fmt.Errorf("invalid inspection port")
	}
	dataDirectory, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dataDirectory = filepath.Join(dataDirectory, "ACURuntime")
	if err := os.MkdirAll(dataDirectory, 0700); err != nil {
		return err
	}
	output, err := os.OpenFile(filepath.Join(dataDirectory, "runtime.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	log.SetOutput(output)
	defer log.SetOutput(os.Stderr)
	icon := desktopIcon()
	var window *application.WebviewWindow
	ready := make(chan struct{})
	show := func() {
		go func() { <-ready; application.InvokeAsync(func() { window.Restore(); window.Show(); window.Focus() }) }()
	}
	windows := application.WindowsOptions{DisableQuitOnLastWindowClosed: true, WebviewUserDataPath: filepath.Join(dataDirectory, "WebView2")}
	if inspectPort != 0 {
		windows.AdditionalBrowserArgs = []string{fmt.Sprintf("--remote-debugging-port=%d", inspectPort), "--remote-debugging-address=127.0.0.1"}
	}
	app := application.New(application.Options{
		Name:                        "ACU Runtime",
		Description:                 "ACU Agent 模型与工具工作区",
		Icon:                        icon,
		Windows:                     windows,
		Mac:                         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		Logger:                      slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelWarn})),
		DisableDefaultSignalHandler: true,
		RawMessageHandler: func(source application.Window, message string, origin *application.OriginInfo) {
			if origin == nil || !strings.HasPrefix(source.Name(), "acu-login-") || len(message) > 32768 {
				return
			}
			var incoming struct {
				Type string `json:"type"`
			}
			if json.Unmarshal([]byte(message), &incoming) != nil || (incoming.Type != "acu-account-login" && incoming.Type != "acu-account-ready" && incoming.Type != "acu-account-refresh" && incoming.Type != "acu-account-refresh-error") {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
				defer cancel()
				top := origin.TopOrigin
				if top == "" && origin.IsMainFrame {
					top = origin.Origin
				}
				if incoming.Type == "acu-account-ready" {
					if err := host.acknowledgeAccountWindow(source.Name(), origin.Origin, top, message); err != nil {
						log.Print("ACU login window handshake rejected")
					}
					return
				}
				result := map[string]any{"ok": true}
				if incoming.Type == "acu-account-refresh" || incoming.Type == "acu-account-refresh-error" {
					if err := host.completeAccountRefresh(ctx, source.Name(), origin.Origin, top, message); err != nil {
						result = map[string]any{"ok": false, "error": "账号暂未完成续期，正在自动重试。"}
					}
					if incoming.Type == "acu-account-refresh" {
						encoded, _ := json.Marshal(result)
						source.ExecJS(`window.dispatchEvent(new CustomEvent('acu-runtime-account-result',{detail:` + string(encoded) + `}))`)
					}
					return
				}
				if err := host.completeAccountLogin(ctx, source.Name(), origin.Origin, top, message); err != nil {
					result = map[string]any{"ok": false, "error": err.Error()}
					host.account.mu.Lock()
					if host.account.ticket.State != "" {
						host.account.view.Error = err.Error()
					}
					host.account.mu.Unlock()
				}
				encoded, _ := json.Marshal(result)
				source.ExecJS(`window.dispatchEvent(new CustomEvent('acu-runtime-account-result',{detail:` + string(encoded) + `}))`)
			}()
		},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.acu.runtime.desktop", OnSecondInstanceLaunch: func(application.SecondInstanceData) { show() }},
		OnShutdown:     shutdown,
	})
	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "acu-main", Title: "ACU Runtime", URL: origin + "?page=overview",
		Width: 1240, Height: 850, MinWidth: 920, MinHeight: 650,
		BackgroundColour: application.NewRGB(24, 25, 26),
		Windows:          application.WindowsWindow{Theme: application.SystemDefault},
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) { event.Cancel(); window.Hide() })
	panel := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "acu-panel", Title: "ACU Runtime · 快捷面板", URL: origin + "/panel",
		Width: 460, Height: 560, MinWidth: 360, MinHeight: 280,
		Hidden: true, Frameless: true, DisableResize: true, AlwaysOnTop: true, HideOnFocusLost: true,
		BackgroundColour: application.NewRGB(24, 25, 26),
		Windows:          application.WindowsWindow{Theme: application.SystemDefault, HiddenOnTaskbar: true},
	})
	panel.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) { event.Cancel(); panel.Hide() })
	panel.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		panel.ExecJS(`window.dispatchEvent(new CustomEvent('acu:visibility',{detail:true}))`)
	})
	panel.OnWindowEvent(events.Common.WindowHide, func(*application.WindowEvent) {
		panel.ExecJS(`window.dispatchEvent(new CustomEvent('acu:visibility',{detail:false}))`)
	})
	close(ready)
	menu := app.NewMenu()
	menu.Add("打开 ACU Runtime").OnClick(func(*application.Context) { show() })
	menu.AddSeparator()
	menu.Add("v0.1.21 · 关闭窗口后保留后台服务").SetEnabled(false)
	menu.Add("退出 ACU Runtime").OnClick(func(*application.Context) { app.Quit() })
	tray := app.SystemTray.New()
	tray.SetIcon(icon)
	tray.SetTooltip("ACU Runtime · 单击快捷面板，右键打开主窗口")
	tray.SetMenu(menu)
	tray.AttachWindow(panel).WindowOffset(8)
	app.Event.RegisterApplicationEventHook(events.Mac.ApplicationShouldHandleReopen, func(event *application.ApplicationEvent) { event.Cancel(); show() })
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case action := <-host.desktopActions:
				application.InvokeAsync(func() {
					switch action.Action {
					case "panel":
						tray.ShowWindow()
					case "hide":
						panel.Hide()
					case "main":
						panel.Hide()
						query, _ := url.ParseQuery(action.Query)
						view := query.Get("view")
						if view == "" || view == "overview" || view == "agents" {
							window.SetURL(origin + "?page=overview")
						} else {
							query.Set("native", view)
							window.SetURL(origin + "?" + query.Encode())
						}
						window.Restore()
						window.Show()
						window.Focus()
					case "fit":
						height := min(max(action.Height, 360), 760)
						width := 460
						if screen, err := panel.GetScreen(); err == nil && screen != nil {
							height = min(height, max(280, screen.WorkArea.Height-24))
							width = min(width, max(360, screen.WorkArea.Width-24))
						}
						panel.SetSize(width, height)
						if panel.IsVisible() {
							_ = tray.PositionWindow(panel, 8)
						}
					}
				})
			}
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			app.Quit()
		case <-quitRequests:
			app.Quit()
		case <-done:
		}
	}()
	go func() {
		for {
			select {
			case <-done:
				return
			case ticket := <-host.loginRequests:
				if !host.account.pending(ticket.State) {
					continue
				}
				application.InvokeAsync(func() {
					login := app.Window.NewWithOptions(accountWindowOptions(ticket))
					closed := make(chan struct{})
					var closeOnce sync.Once
					login.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
						host.account.cancel(ticket.State)
						closeOnce.Do(func() { close(closed) })
					})
					go func() {
						started := time.Now()
						hidden := false
						ticker := time.NewTicker(time.Second)
						defer ticker.Stop()
						for {
							select {
							case <-closed:
								return
							case <-done:
								return
							case <-ticker.C:
								if !hidden && time.Since(started) > 15*time.Second {
									host.account.mu.Lock()
									if host.account.ticket.State == ticket.State && !host.account.view.WindowReady && host.account.view.Error == "" {
										host.account.view.Error = "登录窗口未连接应用，请重试登录或直接填 Key。"
									}
									host.account.mu.Unlock()
								}
								if !host.account.pending(ticket.State) {
									if !host.account.sessionActive(ticket.State) {
										application.InvokeAsync(func() { login.Close() })
										return
									}
									if !hidden {
										hidden = true
										application.InvokeAsync(func() { login.Hide(); window.Restore(); window.Show(); window.Focus() })
									}
									if host.account.refreshDue(ticket.State) && time.Since(started) >= 15*time.Second {
										login.ExecJS(`window.dispatchEvent(new Event('acu-runtime-account-refresh'))`)
										started = time.Now()
									}
								}
							}
						}
					}()
				})
			}
		}
	}()
	log.Print("ACU Runtime native desktop v0.1.21 starting")
	return app.Run()
}

func accountWindowOptions(ticket loginTicket) application.WebviewWindowOptions {
	path := "/sign-in"
	if ticket.Register {
		path = "/sign-up"
	}
	address, _ := json.Marshal(ticket.Site + path)
	encoded, _ := json.Marshal(map[string]any{"site": ticket.Site, "state": ticket.State, "expiresAt": ticket.Expires.Unix()})
	script, _ := assets.ReadFile("web/account-login.js")
	bootstrap := `<!doctype html><meta charset="utf-8"><title>正在打开 ACU 登录</title><body>正在打开 ACU 登录…<script>const bridge=window.chrome?.webview||window.webkit?.messageHandlers?.external;if(bridge){bridge.postMessage('wails:runtime:ready')}location.replace(` + string(address) + `)</script>`
	return application.WebviewWindowOptions{Name: "acu-login-" + ticket.State, Title: "登录 ACU · 完成后自动返回", HTML: bootstrap, JS: strings.Replace(string(script), "__ACU_LOGIN_TICKET__", string(encoded), 1), Width: 1060, Height: 800, MinWidth: 720, MinHeight: 600, Windows: application.WindowsWindow{Theme: application.SystemDefault}}
}
