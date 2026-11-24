import {
  Button,
  Column,
  Empty,
  Grid,
  Image,
  Row,
  Skeleton,
  Text,
  useEffect,
  useState,
} from 'lunabox';
import { isMediaSensitive } from '../format';
import type { DetailStore } from '../store';
import type { SlotRenderProps } from '../types';

export function createScreenshotsTab(store: DetailStore) {
  return function ScreenshotsTab({ game, blurNsfw }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const shouldBlurNsfw = Boolean(blurNsfw);
    const gameId = game?.id ? String(game.id) : '';

    const [revealNsfw, setRevealNsfw] = useState(false);
    const [revealedMap, setRevealedMap] = useState<Record<number, boolean>>({});

    useEffect(() => {
      setRevealNsfw(false);
      setRevealedMap({});
    }, [gameId]);

    if (state.status === 'loading' && !detail) {
      return (
        <Column k="ss-loading" gap="md" width="full">
          <Text text="正在加载游戏截图…" size="sm" tone="muted" />
          <Skeleton lines={5} height={18} />
        </Column>
      );
    }

    if (!detail) {
      return (
        <Empty
          k="ss-unmatched"
          icon="image-off"
          title={state.status === 'error' ? '获取 Hikarinagi 截图失败' : '未匹配到 Hikarinagi 条目'}
          description={
            state.error || '尚未匹配到对应的 Hikarinagi 条目，请点击重试。'
          }
          actionLabel="重新匹配"
          onAction={() => store.loadGame(game, { force: true })}
        />
      );
    }

    const images = Array.isArray(detail.images) ? detail.images.filter((i) => i && i.url) : [];
    const sensitiveCount = images.filter(isMediaSensitive).length;

    return (
      <Column k={`ss-root-${gameId}`} gap={24} width="full">
        {/* 顶栏（无外框，清爽标题 + 操作） */}
        <Row justify="between" align="center" width="full">
          <Row gap="sm" align="center">
            <Text text="游戏截图" size="lg" weight="semibold" tone="strong" />
            <Text text={`共 ${images.length} 张`} size="xs" tone="muted" />
          </Row>
          <Row gap="xs" align="center">
            {shouldBlurNsfw && sensitiveCount > 0 ? (
              <Button
                k="ss-toggle-nsfw"
                label={revealNsfw ? '隐藏敏感截图' : '显示敏感截图'}
                icon="eye-outline"
                variant="ghost"
                size="sm"
                onPress={() => setRevealNsfw((v) => !v)}
              />
            ) : null}
            <Button
              k="ss-refresh"
              label={state.refreshing ? '刷新中…' : '刷新'}
              icon="refresh"
              variant="ghost"
              size="sm"
              loading={state.refreshing}
              onPress={() => store.loadGame(game, { force: true })}
            />
          </Row>
        </Row>

        {images.length === 0 ? (
          <Empty
            icon="image-off"
            title="暂无游戏截图"
            description="该条目在 Hikarinagi 上暂未收录截图。"
          />
        ) : (
          <Grid k="ss-grid" cols={2} gap="md">
            {images.map((img, idx) => {
              const sensitive = isMediaSensitive(img);
              const blurred = shouldBlurNsfw && sensitive && !revealNsfw && !revealedMap[idx];
              const dimLabel =
                img.width && img.height ? `${img.width} × ${img.height}` : `#${idx + 1}`;
              return (
                <Column key={`ss-item-${idx}`} gap="xs">
                  <Image
                    k={`ss-img-${idx}`}
                    src={img.url}
                    height={216}
                    radius={10}
                    fit="cover"
                    blur={blurred}
                    previewable={!blurred}
                    onPress={
                      blurred
                        ? () => setRevealedMap((prev) => ({ ...prev, [idx]: true }))
                        : undefined
                    }
                  />
                  <Row justify="between" align="center" width="full">
                    <Text
                      text={blurred ? '敏感画面（点击解除模糊）' : `截图 ${idx + 1}`}
                      size="xs"
                      tone={blurred ? 'warning' : 'faint'}
                    />
                    <Text text={dimLabel} size="xs" tone="faint" />
                  </Row>
                </Column>
              );
            })}
          </Grid>
        )}
      </Column>
    );
  };
}
