import { Badge, Button, Column, Divider, NativeSlot, Row, Skeleton, Text, useEffect, useState } from 'lunabox';
import type { DetailStore } from '../store';
import type { SlotRenderProps } from '../types';

export function createOverviewTab(store: DetailStore) {
  return function OverviewTab({ game }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : '';

    const [introLang, setIntroLang] = useState<'zh' | 'origin'>('zh');
    useEffect(() => {
      setIntroLang('zh');
    }, [gameId]);

    const transIntro = String(detail?.trans_intro || '').trim();
    const originIntro = String(detail?.origin_intro || '').trim();
    const hasBothIntros = Boolean(transIntro && originIntro && transIntro !== originIntro);
    const activeIntro =
      introLang === 'origin' ? originIntro || transIntro : transIntro || originIntro;

    const sortedTags = Array.isArray(detail?.tags)
      ? detail!.tags
          .filter((t) => t && t.name)
          .slice()
          .sort((a, b) => Number(b.likes || 0) - Number(a.likes || 0))
          .slice(0, 24)
      : [];

    const aliases = Array.isArray(detail?.aliases)
      ? detail!.aliases.map((a) => String(a || '').trim()).filter(Boolean)
      : [];

    return (
      <Column k={`overview-${gameId}`} gap={24} width="full">
        {/* 游戏简介区（无边框，复刻 Hikarinagi 的干净排版 + 中文/原文切换） */}
        <Column k="ov-intro-sec" gap="md" width="full">
          <Row justify="between" align="center" width="full">
            <Text text="简介" size="lg" weight="semibold" tone="strong" />
            {hasBothIntros ? (
              <Row gap="xs" align="center">
                <Button
                  k="ov-lang-zh"
                  label="中文"
                  size="sm"
                  variant={introLang === 'zh' ? 'secondary' : 'ghost'}
                  onPress={() => setIntroLang('zh')}
                />
                <Button
                  k="ov-lang-origin"
                  label="原文"
                  size="sm"
                  variant={introLang === 'origin' ? 'secondary' : 'ghost'}
                  onPress={() => setIntroLang('origin')}
                />
              </Row>
            ) : null}
          </Row>

          {state.status === 'loading' && !detail && !game?.summary ? (
            <Skeleton lines={4} height={14} />
          ) : activeIntro ? (
            <Text text={activeIntro} size="sm" tone="default" selectable={true} />
          ) : (
            <NativeSlot k="ov-native-summary" name="summary" />
          )}
        </Column>

        <Divider subtle={true} />

        {/* 游戏标签区（无缝保留原生可点击筛选与可编辑标签 + Hikarinagi 社区标签） */}
        <Column k="ov-tags-sec" gap="md" width="full">
          <Text text="标签" size="lg" weight="semibold" tone="strong" />
          <NativeSlot k="ov-native-tags" name="tags" />

          {sortedTags.length > 0 ? (
            <Column k="ov-hn-tags" gap="sm" width="full" marginTop="xs">
              <Text
                text={`Hikarinagi 社区标签 (${sortedTags.length})`}
                size="xs"
                weight="medium"
                tone="muted"
              />
              <Row gap="xs" wrap={true} width="full">
                {sortedTags.map((tg, idx) => (
                  <Badge
                    key={`ov-htag-${idx}`}
                    label={Number(tg.likes) > 0 ? `${tg.name} · ${tg.likes}` : tg.name}
                    tone={idx < 4 ? 'brand' : 'default'}
                  />
                ))}
              </Row>
            </Column>
          ) : null}
        </Column>

        {/* 别名区（有数据时展示，无框轻量排布） */}
        {aliases.length > 0 ? (
          <>
            <Divider subtle={true} />
            <Column k="ov-aliases-sec" gap="sm" width="full">
              <Text text="别名" size="base" weight="semibold" tone="strong" />
              <Row gap="sm" wrap={true} width="full">
                {aliases.map((al, idx) => (
                  <Text
                    key={`ov-alias-${idx}`}
                    text={idx < aliases.length - 1 ? `${al}  /` : al}
                    size="sm"
                    tone="muted"
                    selectable={true}
                  />
                ))}
              </Row>
            </Column>
          </>
        ) : null}
      </Column>
    );
  };
}
