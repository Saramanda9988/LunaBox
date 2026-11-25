package service

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"lunabox/backend/applog"
	"lunabox/backend/common/hooks"
	"lunabox/backend/models"
	launcherpkg "lunabox/backend/service/launcher"
)

// 启动流程的插件拦截器（launch/prepare、launch/plan）与运行通知（game/running）。
// 拦截器的超时由各插件注册时声明，这里不再套总超时：需要等用户选择的拦截器（如 Steam 账号选择）可能要等很久，
// 界面侧 StartGame 本来就在后台执行，并显示“启动中”。

const defaultPluginCancelReason = "启动被插件取消"

func interceptCancelReason(outcome hooks.InterceptOutcome) string {
	if reason := strings.TrimSpace(outcome.Reason); reason != "" {
		return reason
	}
	return defaultPluginCancelReason
}

// interceptLaunchPrepare 在解析启动方式之前调用 launch/prepare，返回插件改写后的启动选项。
func (s *StartService) interceptLaunchPrepare(bus hooks.Bus, game models.Game, options launcherpkg.LaunchOptions) (launcherpkg.LaunchOptions, bool, string) {
	raw, err := bus.Intercept(s.ctx, hooks.EventLaunchPrepare, hooks.LaunchPreparePayload{
		Game:    game,
		Options: toHookLaunchOptions(options),
	})
	if err != nil || len(raw) == 0 {
		if err != nil {
			applog.LogWarningf(s.ctx, "launch/prepare interceptors failed: %v", err)
		}
		return options, false, ""
	}
	var res hooks.LaunchPreparePayload
	if err := json.Unmarshal(raw, &res); err != nil {
		applog.LogWarningf(s.ctx, "launch/prepare returned an invalid payload: %v", err)
		return options, false, ""
	}
	if res.Cancel {
		return options, true, interceptCancelReason(res.InterceptOutcome)
	}
	return fromHookLaunchOptions(res.Options), false, ""
}

// interceptLaunchPlan 在启动计划生成之后、启动进程之前调用 launch/plan，把插件改写的字段写回 plan。
func (s *StartService) interceptLaunchPlan(bus hooks.Bus, game models.Game, plan *launcherpkg.LaunchPlan) (bool, string) {
	raw, err := bus.Intercept(s.ctx, hooks.EventLaunchPlan, hooks.LaunchPlanPayload{
		Game: game,
		Plan: toHookLaunchPlan(*plan),
	})
	if err != nil || len(raw) == 0 {
		if err != nil {
			applog.LogWarningf(s.ctx, "launch/plan interceptors failed: %v", err)
		}
		return false, ""
	}
	var res hooks.LaunchPlanPayload
	if err := json.Unmarshal(raw, &res); err != nil {
		applog.LogWarningf(s.ctx, "launch/plan returned an invalid payload: %v", err)
		return false, ""
	}
	if res.Cancel {
		return true, interceptCancelReason(res.InterceptOutcome)
	}
	applyHookLaunchPlan(plan, res.Plan)
	return false, ""
}

func (s *StartService) emitGameRunning(session *activePlaySession, pid uint32, processName, reason string) {
	bus := hooks.SafeBus(s.hookBus)
	if !bus.HasListeners(hooks.EventGameRunning) {
		return
	}
	bus.Emit(hooks.EventGameRunning, hooks.GameRunningPayload{
		Game:        session.game,
		SessionID:   session.sessionID,
		PID:         pid,
		ProcessName: processName,
		Reason:      reason,
	})
}

func toHookLaunchOptions(o launcherpkg.LaunchOptions) hooks.LaunchOptions {
	return hooks.LaunchOptions{
		UseLocaleEmulator: o.UseLocaleEmulator,
		UseMagpie:         o.UseMagpie,
		RunAsAdmin:        o.RunAsAdmin,
		WineRunner:        o.WineRunner,
		WineArgs:          o.WineArgs,
		WinePrefix:        o.WinePrefix,
		UseSteam:          o.UseSteam,
		UseCompatibility:  o.UseCompatibility,
	}
}

func fromHookLaunchOptions(o hooks.LaunchOptions) launcherpkg.LaunchOptions {
	return launcherpkg.LaunchOptions{
		UseLocaleEmulator: o.UseLocaleEmulator,
		UseMagpie:         o.UseMagpie,
		RunAsAdmin:        o.RunAsAdmin,
		WineRunner:        o.WineRunner,
		WineArgs:          o.WineArgs,
		WinePrefix:        o.WinePrefix,
		UseSteam:          o.UseSteam,
		UseCompatibility:  o.UseCompatibility,
	}
}

func toHookLaunchPlan(p launcherpkg.LaunchPlan) hooks.LaunchPlan {
	env := make(map[string]string, len(p.Env))
	for _, kv := range p.Env {
		k, v, _ := strings.Cut(kv, "=")
		if k != "" {
			env[k] = v
		}
	}
	args := p.Args
	if args == nil {
		args = []string{}
	}
	return hooks.LaunchPlan{
		File:       p.File,
		Args:       slices.Clone(args),
		Dir:        p.Dir,
		Env:        env,
		RunAsAdmin: p.RunAsAdmin,
	}
}

// applyHookLaunchPlan 写回插件可改的字段；file 改成空值时保留原值，避免把计划改坏。
func applyHookLaunchPlan(dst *launcherpkg.LaunchPlan, src hooks.LaunchPlan) {
	if file := strings.TrimSpace(src.File); file != "" && file != dst.File {
		dst.File = file
		// 显示名跟随新的启动文件，后面的默认值逻辑会补上
		dst.DisplayName = ""
	}
	dst.Args = slices.Clone(src.Args)
	dst.Dir = src.Dir
	dst.RunAsAdmin = src.RunAsAdmin
	keys := make([]string, 0, len(src.Env))
	for k := range src.Env {
		if strings.TrimSpace(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+src.Env[k])
	}
	dst.Env = env
}
