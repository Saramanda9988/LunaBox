//go:build linux

package integrator

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	steamRestartCommandTimeout = 10 * time.Second
	steamRestartStopTimeout    = 20 * time.Second
)

type steamClientCommand struct {
	name string
	args []string
}

func restartSteamPlatformClient(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	steamRoot, err := findSteamRoot()
	if err != nil {
		return fmt.Errorf("Steam is not installed: %w", err)
	}

	if isSteamRunning() {
		if err := stopSteamClient(ctx, steamRoot); err != nil {
			return err
		}
	}
	return startSteamClient(ctx, steamRoot)
}

func stopSteamClient(ctx context.Context, steamRoot string) error {
	commands := steamClientCommandCandidates(steamRoot, true)
	if len(commands) == 0 {
		return fmt.Errorf("未找到可用的 Steam 关闭命令")
	}

	var errs []error
	for _, candidate := range commands {
		commandCtx, cancel := context.WithTimeout(ctx, steamRestartCommandTimeout)
		cmd := exec.CommandContext(commandCtx, candidate.name, candidate.args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			cancel()
			errs = append(errs, fmt.Errorf(
				"%s: %w: %s",
				candidate.String(),
				err,
				strings.TrimSpace(string(output)),
			))
			continue
		}
		cancel()
		if waitForSteamStopped(steamRestartStopTimeout) {
			return nil
		}
		errs = append(errs, fmt.Errorf("%s: 等待 Steam 退出超时", candidate.String()))
	}

	if !isSteamRunning() {
		return nil
	}
	return fmt.Errorf("关闭 Steam 失败: %w", errors.Join(errs...))
}

func startSteamClient(ctx context.Context, steamRoot string) error {
	commands := steamClientCommandCandidates(steamRoot, false)
	if len(commands) == 0 {
		return fmt.Errorf("未找到可用的 Steam 启动命令")
	}

	var errs []error
	for _, candidate := range commands {
		cmd := exec.CommandContext(ctx, candidate.name, candidate.args...)
		if err := cmd.Start(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", candidate.String(), err))
			continue
		}
		if cmd.Process != nil {
			_ = cmd.Process.Release()
		}
		return nil
	}
	return fmt.Errorf("启动 Steam 失败: %w", errors.Join(errs...))
}

func steamClientCommandCandidates(steamRoot string, shutdown bool) []steamClientCommand {
	flatpakArgs := []string{"run", "com.valvesoftware.Steam"}
	snapArgs := []string{"run", "steam"}
	if shutdown {
		flatpakArgs = append(flatpakArgs, "-shutdown")
		snapArgs = append(snapArgs, "-shutdown")
	}

	systemArgs := []string{}
	if shutdown {
		systemArgs = []string{"-shutdown"}
	}

	isFlatpak := strings.Contains(steamRoot, "/.var/app/com.valvesoftware.Steam/")
	isSnap := strings.Contains(steamRoot, "/snap/steam/")
	commands := make([]steamClientCommand, 0, 3)
	seen := make(map[string]bool)
	add := func(name string, args []string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		path, err := exec.LookPath(name)
		if err != nil {
			return
		}
		key := path + "\x00" + strings.Join(args, "\x00")
		if seen[key] {
			return
		}
		seen[key] = true
		commands = append(commands, steamClientCommand{name: name, args: args})
	}

	if isFlatpak {
		add("flatpak", flatpakArgs)
		if shutdown {
			add("flatpak", []string{"kill", "com.valvesoftware.Steam"})
		}
	}
	if isSnap {
		add("snap", snapArgs)
	}
	add("steam", systemArgs)
	add(filepath.Join(steamRoot, "steam.sh"), systemArgs)
	add(filepath.Join(steamRoot, "ubuntu12_32", "steam"), systemArgs)
	add("flatpak", flatpakArgs)
	if shutdown {
		add("flatpak", []string{"kill", "com.valvesoftware.Steam"})
	}
	add("snap", snapArgs)
	if !shutdown {
		add("xdg-open", []string{"steam://open/main"})
	}
	return commands
}

func waitForSteamStopped(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isSteamRunning() {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return !isSteamRunning()
}

func (command steamClientCommand) String() string {
	if len(command.args) == 0 {
		return command.name
	}
	return command.name + " " + strings.Join(command.args, " ")
}
