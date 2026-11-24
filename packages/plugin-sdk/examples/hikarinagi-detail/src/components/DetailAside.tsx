import {
  Badge,
  Box,
  Button,
  Column,
  Divider,
  IconButton,
  Row,
  Skeleton,
  Text,
} from 'lunabox';
import type { Context } from 'lunabox';
import {
  buildBangumiUrl,
  buildVndbUrl,
  formatLangLabel,
  formatPrice,
  formatReleaseDate,
  formatStatusLabel,
  SITE_GALGAME_BASE,
} from '../format';
import type { DetailStore } from '../store';
import type { SlotRenderProps } from '../types';

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

export function createDetailAside(ctx: Context, store: DetailStore) {
  return function DetailAside({ game }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : '';

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

    return (
      <Column k={`aside-root-${gameId}`} gap={20} width="full">
        {/* 分区 1：游戏详情信息表（无边框键值对，像 Hikarinagi 官网信息栏一样清爽） */}
        <Column k="aside-info-sec" gap="md" width="full">
          <Row justify="between" align="center" width="full">
            <Row gap="xs" align="center">
              <Text text="游戏信息" size="sm" weight="semibold" tone="strong" />
              {detail?.nsfw ? <Badge label="R-18" tone="danger" /> : null}
              {detail ? <Text text={`#${detail.id}`} size="xs" tone="faint" /> : null}
            </Row>
            {detail ? (
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
            ) : (
              <Row gap="xs" align="center">
                <Text
                  text={state.status === 'error' ? '同步失败' : '未匹配'}
                  size="xs"
                  tone="warning"
                />
                <IconButton
                  k="aside-retry-btn"
                  icon="refresh"
                  size="sm"
                  label="重试匹配"
                  disabled={state.refreshing}
                  onPress={() => store.loadGame(game, { force: true })}
                />
              </Row>
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
                text={state.error || '尚未自动匹配到 Hikarinagi 条目。'}
                size="xs"
                tone={state.status === 'error' ? 'danger' : 'muted'}
              />
              {game?.company ? <InfoRow label="开发" value={game.company} /> : null}
              {game?.release_date ? <InfoRow label="首发" value={game.release_date} /> : null}
            </Column>
          )}
        </Column>

        {/* 分区 2：外部链接（紧凑按钮行，无额外外框） */}
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
      </Column>
    );
  };
}
