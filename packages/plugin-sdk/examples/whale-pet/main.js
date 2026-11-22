import { Box, Image } from 'lunabox';

const SOUND_SETS = {
  duck: {
    press: 'assets/press.mp3',
    release: 'assets/release.mp3',
  },
  fx1: {
    press: 'assets/press2.mp3',
    release: 'assets/release2.mp3',
  },
};

export function apply(ctx) {
  ctx.config.define({
    title: '哦鲸鲸设置',
    fields: [
      {
        key: 'anchor',
        type: 'enum',
        label: '停靠位置',
        default: 'bottom-left',
        options: [
          { value: 'bottom-left', label: '左下角' },
          { value: 'bottom-right', label: '右下角' },
        ],
      },
      {
        key: 'size',
        type: 'number',
        label: '挂件大小（px）',
        default: 136,
        min: 72,
        max: 240,
        step: 8,
      },
      {
        key: 'soundSet',
        type: 'enum',
        label: '点击音效',
        default: 'duck',
        options: [
          { value: 'duck', label: '小黄鸭' },
          { value: 'fx1', label: '音效 2' },
          { value: 'off', label: '静音' },
        ],
      },
      {
        key: 'volume',
        type: 'number',
        label: '音量（%）',
        default: 90,
        min: 0,
        max: 100,
        step: 5,
      },
    ],
  });

  const whaleImg = `${ctx.paths.plugin}/assets/whale.png`;

  function readCfg() {
    const raw = ctx.config.get() || {};
    const anchor = raw.anchor === 'bottom-right' ? 'bottom-right' : 'bottom-left';
    const size = Math.min(240, Math.max(72, Number(raw.size) || 136));
    const soundSet =
      raw.soundSet === 'fx1' || raw.soundSet === 'off' ? raw.soundSet : 'duck';
    const rawVol = raw.volume === undefined ? 90 : Number(raw.volume);
    const volume = Math.min(1, Math.max(0, (Number.isFinite(rawVol) ? rawVol : 90) / 100));
    return { anchor, size, soundSet, volume };
  }

  let pressAt = 0;
  let releaseTimer = null;

  function clearReleaseTimer() {
    if (releaseTimer != null) {
      clearTimeout(releaseTimer);
      releaseTimer = null;
    }
  }

  function playPressSound() {
    const cfg = readCfg();
    clearReleaseTimer();
    pressAt = Date.now();
    if (cfg.soundSet === 'off' || cfg.volume <= 0) return;
    const set = SOUND_SETS[cfg.soundSet] || SOUND_SETS.duck;
    ctx.audio.play(set.press, { volume: cfg.volume });
  }

  function playReleaseSound() {
    const cfg = readCfg();
    clearReleaseTimer();
    if (cfg.soundSet === 'off' || cfg.volume <= 0) return;
    const set = SOUND_SETS[cfg.soundSet] || SOUND_SETS.duck;
    // 短按时稍微延后松手音，与原版 whale.js 一样形成「按压啾—回弹哔」连贯两段音效
    const elapsed = Date.now() - pressAt;
    const delay = Math.max(0, 120 - elapsed);
    if (delay === 0) {
      ctx.audio.play(set.release, { volume: cfg.volume });
    } else {
      releaseTimer = setTimeout(() => {
        releaseTimer = null;
        const latest = readCfg();
        if (latest.soundSet === 'off' || latest.volume <= 0) return;
        const s = SOUND_SETS[latest.soundSet] || SOUND_SETS.duck;
        ctx.audio.play(s.release, { volume: latest.volume });
      }, delay);
    }
  }

  let disposeDec = null;

  function mountDecoration() {
    if (disposeDec) {
      disposeDec();
      disposeDec = null;
    }
    const cfg = readCfg();
    const flipX = cfg.anchor === 'bottom-left';
    disposeDec = ctx.theme.registerDecoration({
      id: 'whale-pet-mascot',
      layer: 'foreground',
      anchor: cfg.anchor,
      offset: { x: 6, y: 0 },
      size: { width: cfg.size, height: cfg.size },
      opacity: 1,
      pointerEvents: 'auto',
      render: () =>
        Box({
          k: 'whale-spring-box',
          width: 'full',
          height: 'full',
          springPress: true,
          squishX: 1.05,
          squishY: 0.88,
          flipX,
          onPressIn: playPressSound,
          onPressOut: playReleaseSound,
          children: [
            Image({
              k: 'whale-img',
              src: whaleImg,
              width: 'full',
              height: 'full',
              fit: 'contain',
              transparent: true,
            }),
          ],
        }),
    });
  }

  mountDecoration();

  ctx.config.onChange(() => {
    mountDecoration();
  });

  ctx.effect(() => () => {
    clearReleaseTimer();
    if (disposeDec) {
      disposeDec();
      disposeDec = null;
    }
  });
}
