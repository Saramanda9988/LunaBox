import { Box, Column, Divider, NativeSlot, Row, Skeleton, Text, useEffect, useState } from 'lunabox';
import type { DetailStore } from '../store';
import type { SlotRenderProps } from '../types';

/** 简介超过这个字数时默认折叠（宿主文本只支持单行截断，这里按字数截取） */
const INTRO_PREVIEW = 220;

/** Hina 风格的分段切换：凹陷轨道 + 选中项浮起的卡片底，选中文字用强调色 */
function Segmented<T extends string>({
  k,
  value,
  items,
  onChange,
}: {
  k: string;
  value: T;
  items: Array<{ key: T; label: string }>;
  onChange: (key: T) => void;
}) {
  return (
    <Row k={k} gap={2} padding={2} radius="md" background="chipBg" align="center">
      {items.map((it) => {
        const active = it.key === value;
        return (
          <Box
            key={`${k}-${it.key}`}
            paddingX={10}
            paddingY={3}
            radius={6}
            background={active ? 'cardBg' : 'transparent'}
            border={active ? 1 : 0}
            borderColor="cardBorder"
            label={it.label}
            onPress={() => onChange(it.key)}
          >
            <Text
              text={it.label}
              size="xs"
              weight={active ? 'semibold' : 'medium'}
              tone={active ? 'accent' : 'muted'}
            />
          </Box>
        );
      })}
    </Row>
  );
}

export function createOverviewTab(store: DetailStore) {
  return function OverviewTab({ game }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : '';

    const [introLang, setIntroLang] = useState<'zh' | 'origin'>('zh');
    const [introOpen, setIntroOpen] = useState(false);
    useEffect(() => {
      setIntroLang('zh');
      setIntroOpen(false);
    }, [gameId]);

    const transIntro = String(detail?.trans_intro || '').trim();
    const originIntro = String(detail?.origin_intro || '').trim();
    const hasBothIntros = Boolean(transIntro && originIntro && transIntro !== originIntro);
    const activeIntro =
      introLang === 'origin' ? originIntro || transIntro : transIntro || originIntro;
    const introChars = Array.from(activeIntro);
    const collapsible = introChars.length > INTRO_PREVIEW;
    const shownIntro =
      collapsible && !introOpen
        ? `${introChars.slice(0, INTRO_PREVIEW).join('').trimEnd()}…`
        : activeIntro;
    const loading = state.status === 'loading' && !detail && !game?.summary;

    return (
      <Column k={`overview-${gameId}`} gap={24} width="full">
        {/* 简介：有 Hikarinagi 简介时由插件渲染（中文 / 原文分段切换 + 展开收起），否则交给宿主原生简介（自带标题） */}
        {loading || activeIntro ? (
          <Column k="ov-intro-sec" gap="md" width="full">
            <Row justify="between" align="center" width="full">
              <Text text="简介" size="lg" weight="semibold" tone="strong" />
              {hasBothIntros ? (
                <Segmented
                  k="ov-lang"
                  value={introLang}
                  items={[
                    { key: 'zh', label: '中文' },
                    { key: 'origin', label: '原文' },
                  ]}
                  onChange={(v) => {
                    setIntroLang(v);
                    setIntroOpen(false);
                  }}
                />
              ) : null}
            </Row>

            {loading ? (
              <Skeleton lines={4} height={14} />
            ) : (
              <Column gap="sm" width="full">
                <Text text={shownIntro} size="sm" tone="default" selectable={true} />
                {collapsible ? (
                  <Box
                    k="ov-intro-toggle"
                    alignSelf="start"
                    label={introOpen ? '收起' : '展开'}
                    onPress={() => setIntroOpen(!introOpen)}
                  >
                    <Text text={introOpen ? '收起' : '展开'} size="sm" tone="accent" />
                  </Box>
                ) : null}
              </Column>
            )}
          </Column>
        ) : (
          <NativeSlot k="ov-native-summary" name="summary" />
        )}

        <Divider subtle={true} />

        {/* 游戏标签区（无缝保留原生可点击筛选与可编辑标签） */}
        <Column k="ov-tags-sec" gap="md" width="full">
          <Text text="标签" size="lg" weight="semibold" tone="strong" />
          <NativeSlot k="ov-native-tags" name="tags" />
        </Column>
      </Column>
    );
  };
}
