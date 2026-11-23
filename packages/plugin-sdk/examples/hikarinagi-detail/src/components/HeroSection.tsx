import { Box, HeroBanner, Image, Text, useEffect, useState } from 'lunabox';
import type { Context } from 'lunabox';
import {
  formatLangLabel,
  formatReleaseDate,
  getPrimaryAndSubTitle,
  isMediaSensitive,
  pickBestBanner,
  pickBestLogo,
  sortHeroImages,
} from '../format';
import type { DetailStore } from '../store';
import type { SlotRenderProps } from '../types';

export function createHeroSection(ctx: Context, store: DetailStore) {
  return function HeroSection({ game, blurNsfw }: SlotRenderProps) {
    const state = store.useGameDetail(game);
    const cfg = store.getConfig();
    const detail = state.detail;
    const shouldBlurNsfw = Boolean(blurNsfw);

    const gameId = game?.id ? String(game.id) : '';
    const [selectedIdx, setSelectedIdx] = useState<number>(-1);

    // 切换游戏时重置选中的头图索引
    useEffect(() => {
      setSelectedIdx(-1);
    }, [gameId]);

    const sortedImages = sortHeroImages(detail?.images, shouldBlurNsfw);
    const defaultBanner = pickBestBanner(detail, game, shouldBlurNsfw);

    const activeImg =
      selectedIdx >= 0 && selectedIdx < sortedImages.length ? sortedImages[selectedIdx] : null;
    const activeBannerUrl = activeImg ? activeImg.url : defaultBanner.url;
    const activeBannerSensitive = activeImg ? isMediaSensitive(activeImg) : defaultBanner.sensitive;

    const logo = detail
      ? pickBestLogo(detail.logos, cfg.preferredLogoLang || 'auto', ctx.i18n.getLanguage())
      : null;
    const { title, subtitle } = getPrimaryAndSubTitle(detail, game);

    // 复刻 Hikarinagi 风格的带图标无框元信息行
    const meta: Array<{ icon: string; text: string }> = [];
    const dev = detail?.developer || game?.company || '';
    if (dev) {
      meta.push({ icon: 'palette', text: dev });
    }
    const relDate = detail ? formatReleaseDate(detail) : game?.release_date || '';
    if (relDate) {
      meta.push({ icon: 'calendar-month-outline', text: relDate });
    }
    const platforms = Array.isArray(detail?.platforms)
      ? detail!.platforms.map((p) => String(p || '').trim()).filter(Boolean)
      : [];
    if (platforms.length > 0) {
      meta.push({ icon: 'gamepad-variant-outline', text: platforms.slice(0, 3).join('、') });
    }
    const rawLangs: string[] = [];
    if (Array.isArray(detail?.languages) && detail!.languages.length > 0) {
      for (const l of detail!.languages) {
        const lbl = formatLangLabel(l);
        if (lbl && !rawLangs.includes(lbl)) rawLangs.push(lbl);
      }
    } else if (detail?.origin_lang) {
      const lbl = formatLangLabel(detail.origin_lang);
      if (lbl) rawLangs.push(lbl);
    }
    if (rawLangs.length > 0) {
      meta.push({ icon: 'earth', text: rawLangs.slice(0, 4).join('、') });
    } else if (detail?.adv_type) {
      meta.push({ icon: 'tag-outline', text: String(detail.adv_type) });
    }

    // 评分行（Hikarinagi 优先，无则回退本地评分）
    const hnScore = Number(detail?.rating?.score || 0);
    const localScore = Number(game?.rating || 0);
    const rating =
      hnScore > 0
        ? {
            score: hnScore,
            total: Number(detail?.rating?.total || 0),
            played: Number(detail?.rating?.played || 0),
          }
        : localScore > 0
          ? { score: localScore, total: 0, played: 0 }
          : undefined;

    const rawHeight = Number(cfg.heroHeight);
    const heroHeight =
      Number.isFinite(rawHeight) && rawHeight >= 320 ? Math.min(580, rawHeight) : 460;

    // 默认绝不虚化头图，仅在用户主动开启「始终模糊头图背景」或开启 NSFW 保护且当前头图含敏感内容时虚化
    const shouldBlurHero =
      Boolean(cfg.blurHeroBanner) || (shouldBlurNsfw && activeBannerSensitive);

    // 底部截图预览条（最多展示 6 张，点击切换头图背景，再次点击可大图预览）
    const stripImages = sortedImages.slice(0, 6);
    const currentEffectiveIdx = selectedIdx >= 0 ? selectedIdx : 0;

    return (
      <HeroBanner
        k={`hero-${gameId}`}
        height={heroHeight}
        banner={activeBannerUrl}
        fallbackBanner={game?.cover_source_url || game?.cover_url || ''}
        logo={logo?.url || ''}
        title={title}
        subtitle={subtitle}
        meta={meta}
        rating={rating}
        blur={shouldBlurHero}
        backSlot="back"
        coverSlot="cover"
        toolbarSlot="toolbar"
      >
        {stripImages.length > 1
          ? [
              ...stripImages.map((img, idx) => {
                const isActive = idx === currentEffectiveIdx;
                const thumbBlur = shouldBlurNsfw && isMediaSensitive(img);
                return (
                  <Box
                    key={`hero-thumb-wrap-${idx}`}
                    radius="sm"
                    opacity={isActive ? 1 : 0.78}
                  >
                    <Image
                      k={`hero-thumb-${idx}`}
                      src={img.url}
                      width={122}
                      height={68}
                      radius={7}
                      fit="cover"
                      blur={thumbBlur}
                      hoverBorder={true}
                      previewable={isActive && !thumbBlur}
                      onPress={
                        isActive
                          ? undefined
                          : () => {
                              setSelectedIdx(idx);
                            }
                      }
                    />
                  </Box>
                );
              }),
              sortedImages.length > stripImages.length ? (
                <Box key="hero-thumb-more" paddingX="sm" paddingY="xs">
                  <Text
                    text={`共 ${sortedImages.length} 张截图`}
                    size="xs"
                    tone="light-muted"
                  />
                </Box>
              ) : null,
            ]
          : null}
      </HeroBanner>
    );
  };
}
