import {
  Badge,
  Box,
  Column,
  Divider,
  Icon,
  IconButton,
  Row,
  Skeleton,
  Text,
  useEffect,
  useState,
} from 'lunabox';
import type { Context, VNode } from 'lunabox';
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

/** 别名默认展示的条数，超出部分折叠在“展开”后面（同 Hikarinagi 官网信息栏） */
const ALIAS_PREVIEW = 3;

/** 信息栏的一行：左侧浅色标签，右侧正文；children 用于链接、别名列表等自定义内容 */
function InfoRow({
  label,
  value,
  children,
}: {
  label: string;
  value?: string | null;
  children?: unknown;
}) {
  const v = String(value || '').trim();
  if (!v && !children) return null;
  return (
    <Row gap="md" align="start" width="full">
      <Box width={44} shrink={0}>
        <Text text={label} size="sm" tone="muted" />
      </Box>
      <Column grow={1} basis={0} minWidth={0} gap={4}>
        {children ? (children as VNode) : <Text text={v} size="sm" tone="default" selectable={true} />}
      </Column>
    </Row>
  );
}

/** 强调色文字链接（展开 / 收起、官网），颜色跟随主题插件的 accentText */
function AccentLink({ k, text, onPress }: { k: string; text: string; onPress: () => void }) {
  return (
    <Box k={k} alignSelf="start" onPress={onPress} label={text}>
      <Text text={text} size="sm" tone="accent" />
    </Box>
  );
}

export function createDetailAside(ctx: Context, store: DetailStore) {
  return function DetailAside({ game }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : '';

    const [aliasesOpen, setAliasesOpen] = useState(false);
    useEffect(() => {
      setAliasesOpen(false);
    }, [gameId]);

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
        label: 'Hikarinagi',
        url: `${SITE_GALGAME_BASE}${detail.id}`,
        icon: 'open-in-new',
      });
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

    const homepage = String(detail?.homepage || '').trim();
    const aliases = Array.isArray(detail?.aliases)
      ? detail!.aliases.map((a) => String(a || '').trim()).filter(Boolean)
      : [];
    const shownAliases = aliasesOpen ? aliases : aliases.slice(0, ALIAS_PREVIEW);

    return (
      <Column k={`aside-root-${gameId}`} gap={20} width="full">
        {/* 分区 1：游戏信息（同 Hikarinagi 官网左侧信息栏：浅色标签 + 正文，别名逐行列出并可展开） */}
        <Column k="aside-info-sec" gap="md" width="full">
          <Row justify="between" align="center" width="full">
            <Row gap="xs" align="center">
              <Text text="游戏信息" size="sm" weight="semibold" tone="strong" />
              {detail?.nsfw ? <Badge label="R-18" tone="danger" /> : null}
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
            <Column k="aside-kv-list" gap={12} width="full">
              <InfoRow label="开发" value={detail.developer} />
              <InfoRow label="发行" value={detail.publisher} />
              <InfoRow label="首发" value={formatReleaseDate(detail)} />
              <InfoRow label="类型" value={detail.adv_type} />
              <InfoRow label="平台" value={platforms.join('、')} />
              <InfoRow label="语言" value={langs.join('、')} />
              <InfoRow label="引擎" value={detail.engine} />
              <InfoRow label="状态" value={formatStatusLabel(detail)} />
              <InfoRow label="分级" value={detail.nsfw ? 'R-18 / 成人向' : '全年龄'} />
              {prices.length > 0 ? (
                <InfoRow label="定价" value={prices.slice(0, 3).join('\n')} />
              ) : null}
              {homepage ? (
                <InfoRow label="官网">
                  <AccentLink
                    k="aside-homepage"
                    text={homepage}
                    onPress={() => ctx.shell.openURL(homepage)}
                  />
                </InfoRow>
              ) : null}
              <InfoRow label="原名" value={detail.origin_title} />
              <InfoRow label="译名" value={detail.trans_title} />
              <InfoRow label="英文" value={detail.en_title} />
              {aliases.length > 0 ? (
                <InfoRow label="别名">
                  {shownAliases.map((al, idx) => (
                    <Text
                      key={`aside-alias-${idx}`}
                      text={al}
                      size="sm"
                      tone="default"
                      selectable={true}
                    />
                  ))}
                  {aliases.length > ALIAS_PREVIEW ? (
                    <AccentLink
                      k="aside-alias-toggle"
                      text={aliasesOpen ? '收起' : `展开（共 ${aliases.length} 个）`}
                      onPress={() => setAliasesOpen(!aliasesOpen)}
                    />
                  ) : null}
                </InfoRow>
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

        {/* 分区 2：相关链接（发丝线描边的小胶囊，悬停换成柔和强调底，颜色全部取主题令牌） */}
        {links.length > 0 ? (
          <>
            <Divider subtle={true} />
            <Column k="aside-links-sec" gap="sm" width="full">
              <Text text="相关链接" size="sm" weight="semibold" tone="strong" />
              <Row gap={6} wrap={true} width="full">
                {links.map((lk, idx) => (
                  <Row
                    key={`ext-link-${idx}`}
                    gap={6}
                    align="center"
                    paddingX={10}
                    paddingY={5}
                    radius="md"
                    background="cardBg"
                    border={1}
                    borderColor="cardBorder"
                    hoverBackground="accentSoft"
                    label={lk.label}
                    onPress={() => ctx.shell.openURL(lk.url)}
                  >
                    <Icon name={lk.icon} size={14} tone="accent" />
                    <Text text={lk.label} size="xs" weight="medium" tone="default" wrap={false} />
                  </Row>
                ))}
              </Row>
            </Column>
          </>
        ) : null}
      </Column>
    );
  };
}
