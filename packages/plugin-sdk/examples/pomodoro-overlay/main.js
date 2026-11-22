import {
  useState,
  useEffect,
  Column,
  Row,
  Badge,
  Button,
  IconButton,
  Progress,
  Text,
} from 'lunabox';

function pad2(n) {
  return String(Math.max(0, Math.floor(n))).padStart(2, '0');
}

function formatPlayClock(totalSec) {
  const sec = Math.max(0, Math.floor(totalSec));
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  return `${pad2(h)}:${pad2(m)}:${pad2(s)}`;
}

function formatCountdown(totalSec) {
  const sec = Math.max(0, Math.floor(totalSec));
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${pad2(m)}:${pad2(s)}`;
}

function formatHumanDuration(totalSec) {
  const sec = Math.max(0, Math.floor(totalSec));
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h} 小时 ${m} 分`;
  if (m > 0) return `${m} 分 ${s} 秒`;
  return `${s} 秒`;
}

const EXPANDED_SIZE = { w: 296, h: 214 };
const COMPACT_SIZE = { w: 248, h: 46 };

export function apply(ctx) {
  ctx.config.define({
    title: '番茄钟悬浮窗设置',
    icon: 'timer-outline',
    fields: [
      {
        key: 'focusMinutes',
        type: 'number',
        label: '专注时长（分钟）',
        default: 25,
        min: 1,
        max: 180,
        step: 5,
      },
      {
        key: 'breakMinutes',
        type: 'number',
        label: '休息时长（分钟）',
        default: 5,
        min: 1,
        max: 60,
        step: 1,
      },
      {
        key: 'autoOpenOnLaunch',
        type: 'boolean',
        label: '游戏启动时自动弹出悬浮窗',
        default: true,
      },
      {
        key: 'autoCloseOnEnd',
        type: 'boolean',
        label: '游戏结束时自动关闭悬浮窗',
        default: false,
      },
    ],
  });

  function readCfg() {
    const raw = ctx.config.get() || {};
    const focusMin = Math.max(1, Number(raw.focusMinutes) || 25);
    const breakMin = Math.max(1, Number(raw.breakMinutes) || 5);
    return {
      focusSec: focusMin * 60,
      breakSec: breakMin * 60,
      autoOpenOnLaunch: raw.autoOpenOnLaunch !== false,
      autoCloseOnEnd: Boolean(raw.autoCloseOnEnd),
    };
  }

  const initCfg = readCfg();

  // 共享状态：同时响应游戏生命周期事件、1 秒定时器与悬浮窗内按钮交互
  const state = {
    playing: false,
    gameName: '',
    sessionId: null,
    gameStartedAt: null,
    playSeconds: 0,
    phase: 'focus', // 'focus' | 'break'
    pomoRunning: false,
    pomoTotal: initCfg.focusSec,
    pomoRemaining: initCfg.focusSec,
    completedCount: 0,
    compact: false,
    tick: 0,
  };

  let overlayWin = null;
  const listeners = new Set();

  function notify() {
    state.tick++;
    for (const fn of listeners) {
      try {
        fn(state.tick);
      } catch (e) {
        ctx.logger.error('notify error:', e);
      }
    }
  }

  function switchPhase(nextPhase, autoStart) {
    const cfg = readCfg();
    const total = nextPhase === 'break' ? cfg.breakSec : cfg.focusSec;
    state.phase = nextPhase;
    state.pomoTotal = total;
    state.pomoRemaining = total;
    if (typeof autoStart === 'boolean') {
      state.pomoRunning = autoStart;
    }
    notify();
  }

  function resetPomodoro() {
    const cfg = readCfg();
    const total = state.phase === 'break' ? cfg.breakSec : cfg.focusSec;
    state.pomoTotal = total;
    state.pomoRemaining = total;
    notify();
  }

  function toggleCompact() {
    state.compact = !state.compact;
    if (overlayWin) {
      const sz = state.compact ? COMPACT_SIZE : EXPANDED_SIZE;
      overlayWin.setSize(sz.w, sz.h);
    }
    notify();
  }

  function closeOverlay() {
    if (overlayWin) {
      const w = overlayWin;
      overlayWin = null;
      w.close();
    }
  }

  function PomodoroOverlayView() {
    const [, setTick] = useState(state.tick);

    useEffect(() => {
      const sub = (t) => setTick(t);
      listeners.add(sub);
      return () => listeners.delete(sub);
    }, []);

    const playClock = formatPlayClock(state.playSeconds);
    const pomoClock = formatCountdown(state.pomoRemaining);
    const progress =
      state.pomoTotal > 0
        ? Math.min(1, Math.max(0, (state.pomoTotal - state.pomoRemaining) / state.pomoTotal))
        : 0;

    if (state.compact) {
      return Row({
        k: 'compact-pill',
        width: 'full',
        height: 'full',
        surface: 'glass',
        radius: 'full',
        border: true,
        padding: 'sm',
        gap: 'xs',
        align: 'center',
        justify: 'between',
        dragWindow: true,
        children: [
          Row({
            k: 'compact-info',
            gap: 'xs',
            align: 'center',
            children: [
              Badge({
                k: 'compact-badge',
                label: state.playing ? '游玩' : state.phase === 'break' ? '休息' : '番茄',
                tone: state.playing ? 'success' : state.phase === 'break' ? 'warning' : 'brand',
              }),
              Text({
                k: 'compact-clock',
                text: `${playClock} · ${pomoClock}`,
                size: 'sm',
                weight: 'semibold',
                tone: 'strong',
                wrap: false,
              }),
            ],
          }),
          Row({
            k: 'compact-actions',
            gap: 'none',
            align: 'center',
            children: [
              IconButton({
                k: 'expand-btn',
                icon: 'chevron-down',
                size: 'sm',
                label: '展开',
                onPress: toggleCompact,
              }),
              IconButton({
                k: 'compact-close',
                icon: 'close',
                size: 'sm',
                label: '关闭',
                onPress: closeOverlay,
              }),
            ],
          }),
        ],
      });
    }

    const statusLabel = state.playing
      ? '游玩中'
      : state.pomoRunning
      ? state.phase === 'break'
        ? '休息中'
        : '专注中'
      : '待机';
    const statusTone = state.playing
      ? 'success'
      : state.pomoRunning
      ? state.phase === 'break'
        ? 'warning'
        : 'brand'
      : 'neutral';
    const titleText = state.gameName || '未启动游戏';
    const phaseLabel =
      (state.phase === 'break' ? '休息倒计时' : '专注倒计时') +
      (state.completedCount > 0 ? ` · 已完成 ${state.completedCount} 轮` : '');

    return Column({
      k: 'overlay-card',
      width: 'full',
      height: 'full',
      surface: 'glass',
      radius: 'lg',
      border: true,
      padding: 'md',
      gap: 'sm',
      dragWindow: true,
      children: [
        // 顶部栏：状态 + 当前游戏名 + 收起/关闭按钮（整条可拖动）
        Row({
          k: 'header',
          width: 'full',
          align: 'center',
          justify: 'between',
          gap: 'xs',
          children: [
            Row({
              k: 'title-group',
              gap: 'xs',
              align: 'center',
              grow: 1,
              children: [
                Badge({ k: 'status-badge', label: statusLabel, tone: statusTone }),
                Text({
                  k: 'game-title',
                  text: titleText,
                  size: 'sm',
                  weight: 'semibold',
                  tone: 'strong',
                  maxLines: 1,
                }),
              ],
            }),
            Row({
              k: 'win-controls',
              gap: 'none',
              align: 'center',
              children: [
                IconButton({
                  k: 'collapse-btn',
                  icon: 'minus',
                  size: 'sm',
                  label: '收起为胶囊',
                  onPress: toggleCompact,
                }),
                IconButton({
                  k: 'close-btn',
                  icon: 'close',
                  size: 'sm',
                  label: '关闭悬浮窗',
                  onPress: closeOverlay,
                }),
              ],
            }),
          ],
        }),

        // 游玩时长展示卡片
        Row({
          k: 'playtime-box',
          width: 'full',
          surface: 'glass-card',
          radius: 'md',
          border: true,
          padding: 'sm',
          align: 'center',
          justify: 'between',
          children: [
            Text({
              k: 'playtime-label',
              text: state.playing ? '本次已游玩' : '上次游玩时长',
              size: 'xs',
              tone: 'muted',
              wrap: false,
            }),
            Text({
              k: 'playtime-value',
              text: playClock,
              size: 'lg',
              weight: 'bold',
              tone: state.playing ? 'brand' : 'strong',
              wrap: false,
            }),
          ],
        }),

        // 番茄钟倒计时与进度条
        Column({
          k: 'pomo-section',
          width: 'full',
          gap: 'xs',
          children: [
            Row({
              k: 'pomo-row',
              width: 'full',
              align: 'center',
              justify: 'between',
              children: [
                Text({ k: 'pomo-phase', text: phaseLabel, size: 'xs', tone: 'muted', wrap: false }),
                Text({
                  k: 'pomo-clock',
                  text: pomoClock,
                  size: 'base',
                  weight: 'semibold',
                  tone: state.phase === 'break' ? 'success' : 'strong',
                  wrap: false,
                }),
              ],
            }),
            Progress({
              k: 'pomo-progress',
              value: progress,
              tone: state.phase === 'break' ? 'success' : 'brand',
            }),
          ],
        }),

        // 底部控制按钮
        Row({
          k: 'controls',
          width: 'full',
          align: 'center',
          justify: 'between',
          gap: 'xs',
          children: [
            Row({
              k: 'controls-left',
              gap: 'xs',
              align: 'center',
              children: [
                Button({
                  k: 'toggle-pomo',
                  icon: state.pomoRunning ? 'pause' : 'play',
                  label: state.pomoRunning ? '暂停' : '开始',
                  variant: state.pomoRunning ? 'secondary' : 'primary',
                  size: 'sm',
                  onPress: () => {
                    state.pomoRunning = !state.pomoRunning;
                    notify();
                  },
                }),
                Button({
                  k: 'switch-phase',
                  label: state.phase === 'focus' ? '休息' : '专注',
                  variant: 'ghost',
                  size: 'sm',
                  onPress: () => switchPhase(state.phase === 'focus' ? 'break' : 'focus', true),
                }),
              ],
            }),
            IconButton({
              k: 'reset-pomo',
              icon: 'restart',
              size: 'sm',
              label: '重置番茄钟',
              onPress: resetPomodoro,
            }),
          ],
        }),
      ],
    });
  }

  function openOverlay() {
    if (overlayWin) {
      overlayWin.show();
      notify();
      return;
    }
    const sz = state.compact ? COMPACT_SIZE : EXPANDED_SIZE;
    overlayWin = ctx.window.open('pomodoro', {
      title: '番茄钟悬浮窗',
      width: sz.w,
      height: sz.h,
      frameless: true,
      transparent: true,
      alwaysOnTop: true,
      skipTaskbar: true,
      resizable: false,
      render: PomodoroOverlayView,
    });
    overlayWin.onClose(() => {
      overlayWin = null;
    });
  }

  // 每秒刷新游玩时长与番茄钟倒计时
  const timerId = setInterval(() => {
    let changed = false;
    if (state.playing && state.gameStartedAt != null) {
      const elapsed = Math.max(0, Math.floor((Date.now() - state.gameStartedAt) / 1000));
      if (elapsed !== state.playSeconds) {
        state.playSeconds = elapsed;
        changed = true;
      }
    }
    if (state.pomoRunning) {
      state.pomoRemaining = Math.max(0, state.pomoRemaining - 1);
      changed = true;
      if (state.pomoRemaining <= 0) {
        const cfg = readCfg();
        if (state.phase === 'focus') {
          state.completedCount++;
          state.phase = 'break';
          state.pomoTotal = cfg.breakSec;
          state.pomoRemaining = cfg.breakSec;
          ctx.ui.toast(
            `专注完成！休息 ${Math.round(cfg.breakSec / 60)} 分钟吧（已玩 ${formatPlayClock(state.playSeconds)}）`,
            'success'
          );
        } else {
          state.phase = 'focus';
          state.pomoTotal = cfg.focusSec;
          state.pomoRemaining = cfg.focusSec;
          ctx.ui.toast('休息结束，开始新一轮番茄钟', 'info');
        }
      }
    }
    if (changed && overlayWin) {
      notify();
    }
  }, 1000);

  ctx.effect(() => () => {
    clearInterval(timerId);
  });

  // 游戏启动：记录开始时间、启动番茄钟并自动弹出悬浮窗
  ctx.on('game/launched', (payload) => {
    const game = payload && payload.game ? payload.game : null;
    const cfg = readCfg();
    state.playing = true;
    state.gameName = (game && game.name) || '正在游玩';
    state.sessionId = (payload && payload.sessionId) || null;
    state.gameStartedAt = Date.now();
    state.playSeconds = 0;
    state.phase = 'focus';
    state.pomoTotal = cfg.focusSec;
    state.pomoRemaining = cfg.focusSec;
    state.pomoRunning = true;
    if (cfg.autoOpenOnLaunch) {
      openOverlay();
    } else {
      notify();
    }
  });

  // 游戏结束：定格游玩时长，并按设置决定是否自动关闭悬浮窗
  ctx.on('session/ended', (payload) => {
    const sid = payload && payload.sessionId;
    if (state.sessionId && sid && sid !== state.sessionId) {
      return;
    }
    const finalSec =
      payload && typeof payload.duration === 'number' && payload.duration > 0
        ? payload.duration
        : state.playSeconds;
    state.playing = false;
    state.gameStartedAt = null;
    state.sessionId = null;
    state.playSeconds = finalSec;
    state.pomoRunning = false;

    if (state.gameName) {
      ctx.ui.toast(`本次游玩《${state.gameName}》共 ${formatHumanDuration(finalSec)}`, 'info');
    }
    if (readCfg().autoCloseOnEnd) {
      closeOverlay();
    } else {
      notify();
    }
  });

  // 顶栏时钟图标：可随时一键开关悬浮窗进行预览或手动使用番茄钟
  ctx.ui.slot('topbar.item', {
    id: 'toggle-pomodoro',
    title: '番茄钟悬浮窗',
    icon: 'timer-outline',
    onPress: () => {
      if (overlayWin) {
        closeOverlay();
      } else {
        openOverlay();
      }
    },
  });
}
