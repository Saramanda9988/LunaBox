import {
  Badge,
  Box,
  Button,
  Collapsible,
  Column,
  Divider,
  Icon,
  IconButton,
  Input,
  Progress,
  Row,
  Skeleton,
  Text,
  useEffect,
  useState,
} from 'lunabox';
import type { Context } from 'lunabox';
import {
  buildBangumiUrl,
  buildVndbUrl,
  formatLangLabel,
  formatPrice,
  formatRelativeTime,
  formatReleaseDate,
  formatStatusLabel,
  normalizeHikarinagiId,
  SITE_GALGAME_BASE,
} from '../format';
import type { DetailStore } from '../store';
import type { HikarinagiRating, SlotRenderProps } from '../types';

function InfoRow({
  label,
  value,
}: {
  label: string;
  value?: string | null;
}) {
  const v = String(value || '').trim();
  if (!v) return null;
  return (
    <Row gap="sm" align="start" width="full">
      <Box width={64} shrink={0}>
        <Text text={label} size="xs" tone="muted" />
      </Box>
      <Box grow={1} basis={0} minWidth={0}>
        <Text text={v} size="xs" tone="strong" selectable={true} />
      </Box>
    </Row>
  );
}

function RatingStars({ score }: { score: number }) {
  const stars = score / 2;
  return (
    <Row gap={2} align="center">
      {[1, 2, 3, 4, 5].map((i) => {
        let name = 'star-outline';
        let tone: 'warning' | 'faint' = 'faint';
        if (stars >= i - 0.25) {
          name = 'star';
          tone = 'warning';
        } else if (stars >= i - 0.75) {
          name = 'star-half-full';
          tone = 'warning';
        }
        return <Icon key={`st-${i}`} name={name} size={15} tone={tone} />;
      })}
    </Row>
  );
}

function RatingDistribution({ rating }: { rating?: HikarinagiRating }) {
  if (!rating?.count || Number(rating.total || 0) <= 0) return null;
  const total = Number(rating.total);
  const c = rating.count;
  const buckets = [
    { label: '9-10', n: Number(c['10'] || 0) + Number(c['9'] || 0) },
    { label: '7-8', n: Number(c['8'] || 0) + Number(c['7'] || 0) },
    { label: '5-6', n: Number(c['6'] || 0) + Number(c['5'] || 0) },
    { label: '3-4', n: Number(c['4'] || 0) + Number(c['3'] || 0) },
    { label: '1-2', n: Number(c['2'] || 0) + Number(c['1'] || 0) },
  ];
  return (
    <Column k="aside-rating-dist" gap={6} width="full" marginTop="xs">
      {buckets.map((b, idx) => {
        const pct = total > 0 ? Math.round((b.n / total) * 100) : 0;
        return (
          <Row key={`rd-${idx}`} gap="sm" align="center" width="full">
            <Box width={34} shrink={0}>
              <Text text={b.label} size="xs" tone="faint" />
            </Box>
            <Box grow={1} basis={0} minWidth={0}>
              <Progress value={pct} max={100} height={5} tone={idx === 0 ? 'brand' : 'default'} />
            </Box>
            <Box width={30} shrink={0}>
              <Text text={`${pct}%`} size="xs" tone="faint" />
            </Box>
          </Row>
        );
      })}
    </Column>
  );
}

export function createDetailAside(ctx: Context, store: DetailStore) {
  return function DetailAside({ game }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : '';
    const [bindInput, setBindInput] = useState('');

    useEffect(() => {
      setBindInput('');
    }, [gameId]);

    const handleManualBind = () => {
      const normalized = normalizeHikarinagiId(bindInput);
      if (!normalized) {
        ctx.ui.toast('请输入有效的 Hikarinagi 数字 ID 或条目链接', 'error');
        return;
      }
      setBindInput('');
      store.loadGame(game, { force: true, bindId: normalized });
    };

    if (state.status === 'loading' && !detail) {
      return (
        <Column k="aside-loading" gap="md" width="full">
          <Row justify="between" align="center" width="full">
            <Text text="游戏详情" size="base" weight="semibold" tone="strong" />
            <Text text="同步中…" size="xs" tone="muted" />
          </Row>
          <Skeleton lines={7} height={14} />
        </Column>
      );
    }

    const links: Array<{ label: string; url: string; icon: string }> = [];
    if (detail?.id) {
      links.push({
        label: `Hikarinagi #${detail.id}`,
        url: `${SITE_GALGAME_BASE}${detail.id}`,
        icon: 'open-in-new',
      });
    }
    if (detail?.homepage) {
      links.push({ label: '官方网站', url: String(detail.homepage), icon: 'link-variant' });
    }
    if (Array.isArray(detail?.steam_apps)) {
      detail!.steam_apps.forEach((sa, idx) => {
        if (sa && Number(sa.app_id) > 0) {
          links.push({
            label: idx === 0 ? 'Steam' : `Steam #${idx + 1}`,
            url: `https://store.steampowered.com/app/${sa.app_id}/`,
            icon: 'steam',
          });
        }
      });
    }
    if (detail?.external_source) {
      const bgmUrl = buildBangumiUrl(detail.external_source.bangumi);
      if (bgmUrl) links.push({ label: 'Bangumi', url: bgmUrl, icon: 'open-in-new' });
      const vndbUrl = buildVndbUrl(detail.external_source.vndb);
      if (vndbUrl) links.push({ label: 'VNDB', url: vndbUrl, icon: 'open-in-new' });
    }
    if (Array.isArray(detail?.external_links)) {
      detail!.external_links.forEach((el) => {
        if (el?.url) {
          links.push({
            label: String(el.label || el.name || '外部链接'),
            url: String(el.url),
            icon: 'link-variant',
          });
        }
      });
    }

    const platforms = Array.isArray(detail?.platforms)
      ? detail!.platforms.map((p) => String(p || '').trim()).filter(Boolean)
      : [];
    const prices = Array.isArray(detail?.prices)
      ? detail!.prices.map(formatPrice).filter(Boolean)
      : [];
    const langs: string[] = [];
    if (Array.isArray(detail?.languages) && detail!.languages.length > 0) {
      for (const l of detail!.languages) {
        const lbl = formatLangLabel(l);
        if (lbl && !langs.includes(lbl)) langs.push(lbl);
      }
    } else if (detail?.origin_lang) {
      const lbl = formatLangLabel(detail.origin_lang);
      if (lbl) langs.push(lbl);
    }

    const score = Number(detail?.rating?.score || 0);
    const total = Number(detail?.rating?.total || 0);
    const played = Number(detail?.rating?.played || 0);
    const cacheLabel = state.fetchedAt
      ? `${state.fromCache ? 'SQLite 缓存' : '实时同步'} · ${formatRelativeTime(state.fetchedAt)}`
      : '';

    return (
      <Column k={`aside-root-${gameId}`} gap={20} width="full">
        {/* 分区 1：Hikarinagi 评分（无框通透排布，复刻官网右侧评分模块） */}
        {detail ? (
          <>
            <Column k="aside-rating-sec" gap="sm" width="full">
              <Row justify="between" align="center" width="full">
                <Row gap="xs" align="center">
                  <Text text="社区评分" size="sm" weight="semibold" tone="strong" />
                  {detail.nsfw ? <Badge label="R-18" tone="danger" /> : null}
                </Row>
                <Row gap="xs" align="center">
                  <IconButton
                    k="aside-refresh-btn"
                    icon="refresh"
                    size="sm"
                    label="刷新 Hikarinagi 详情"
                    disabled={state.refreshing}
                    onPress={() => store.loadGame(game, { force: true })}
                  />
                  <IconButton
                    k="aside-open-hn"
                    icon="open-in-new"
                    size="sm"
                    label="在浏览器打开 Hikarinagi"
                    onPress={() => ctx.shell.openURL(`${SITE_GALGAME_BASE}${detail.id}`)}
                  />
                </Row>
              </Row>

              {score > 0 ? (
                <Row gap="md" align="center" width="full">
                  <Text text={score.toFixed(1)} size="3xl" weight="bold" tone="strong" />
                  <Column gap={2}>
                    <RatingStars score={score} />
                    <Text
                      text={
                        [total > 0 ? `${total} 人评分` : '', played > 0 ? `${played} 人玩过` : '']
                          .filter(Boolean)
                          .join(' · ') || 'Hikarinagi 均分'
                      }
                      size="xs"
                      tone="muted"
                    />
                  </Column>
                </Row>
              ) : (
                <Text text="暂无社区评分" size="sm" tone="muted" />
              )}

              <RatingDistribution rating={detail.rating} />

              {cacheLabel ? <Text text={cacheLabel} size="xs" tone="faint" /> : null}
            </Column>
            <Divider subtle={true} />
          </>
        ) : null}

        {/* 分区 2：游戏详情信息表（无边框键值对，像 Hikarinagi 官网信息栏一样清爽） */}
        <Column k="aside-info-sec" gap="md" width="full">
          <Row justify="between" align="center" width="full">
            <Text text="游戏信息" size="sm" weight="semibold" tone="strong" />
            {detail ? (
              <Text text={`#${detail.id}`} size="xs" tone="faint" />
            ) : (
              <Text
                text={state.status === 'error' ? '同步失败' : '未匹配'}
                size="xs"
                tone="warning"
              />
            )}
          </Row>

          {detail ? (
            <Column k="aside-kv-list" gap={10} width="full">
              <InfoRow label="原名" value={detail.origin_title} />
              <InfoRow label="译名" value={detail.trans_title} />
              <InfoRow label="英文名" value={detail.en_title} />
              <InfoRow label="开发" value={detail.developer} />
              <InfoRow label="发行" value={detail.publisher} />
              <InfoRow label="首发" value={formatReleaseDate(detail)} />
              <InfoRow label="状态" value={formatStatusLabel(detail)} />
              <InfoRow label="类型" value={detail.adv_type} />
              <InfoRow label="平台" value={platforms.join('、')} />
              <InfoRow label="语言" value={langs.join('、')} />
              <InfoRow label="引擎" value={detail.engine} />
              <InfoRow label="分级" value={detail.nsfw ? 'R-18 / 成人向' : '全年龄'} />
              {prices.length > 0 ? (
                <InfoRow label="定价" value={prices.slice(0, 3).join('\n')} />
              ) : null}
            </Column>
          ) : (
            <Column gap="sm" width="full">
              <Text
                text={
                  state.error ||
                  '尚未自动匹配到 Hikarinagi 条目，可在下方输入条目 ID 手动关联。'
                }
                size="xs"
                tone={state.status === 'error' ? 'danger' : 'muted'}
              />
              {game?.company ? <InfoRow label="开发" value={game.company} /> : null}
              {game?.release_date ? <InfoRow label="首发" value={game.release_date} /> : null}
            </Column>
          )}
        </Column>

        {/* 分区 3：外部链接（紧凑按钮行，无额外外框） */}
        {links.length > 0 ? (
          <>
            <Divider subtle={true} />
            <Column k="aside-links-sec" gap="sm" width="full">
              <Text text="相关链接" size="sm" weight="semibold" tone="strong" />
              <Row gap="xs" wrap={true} width="full">
                {links.map((lk, idx) => (
                  <Button
                    key={`ext-link-${idx}`}
                    label={lk.label}
                    icon={lk.icon}
                    variant="secondary"
                    size="sm"
                    onPress={() => ctx.shell.openURL(lk.url)}
                  />
                ))}
              </Row>
            </Column>
          </>
        ) : null}

        {/* 分区 4：轻量级条目关联与缓存管理（默认收起，不干扰主视觉） */}
        <Divider subtle={true} />
        <Collapsible
          k="aside-bind-col"
          title="条目关联与缓存"
          icon="database-refresh-outline"
          defaultOpen={!detail}
        >
          <Column gap="sm" width="full" paddingY="xs">
            <Text
              text="输入 Hikarinagi 数字 ID 或详情页链接："
              size="xs"
              tone="muted"
            />
            <Row gap="xs" align="center" width="full">
              <Box grow={1} basis={0} minWidth={0}>
                <Input
                  k="aside-bind-input"
                  value={bindInput}
                  placeholder="例如 25423 或条目 URL"
                  onChange={setBindInput}
                  onSubmit={handleManualBind}
                />
              </Box>
              <Button
                k="aside-bind-submit"
                label="绑定"
                variant="primary"
                size="sm"
                loading={state.refreshing}
                onPress={handleManualBind}
              />
            </Row>
            <Row gap="xs" wrap={true}>
              <Button
                k="aside-force-refresh"
                label="强制刷新"
                icon="refresh"
                variant="ghost"
                size="sm"
                loading={state.refreshing}
                onPress={() => store.loadGame(game, { force: true })}
              />
              {state.manualBound ? (
                <Button
                  k="aside-unbind"
                  label="取消手动绑定"
                  icon="restore"
                  variant="ghost"
                  size="sm"
                  onPress={() => store.unbindGame(game)}
                />
              ) : null}
            </Row>
          </Column>
        </Collapsible>
      </Column>
    );
  };
}
