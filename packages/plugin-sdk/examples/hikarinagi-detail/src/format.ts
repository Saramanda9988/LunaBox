import type { Game } from 'lunabox';
import type {
  BannerPick,
  HikarinagiGalgameDetail,
  HikarinagiLogo,
  HikarinagiMedia,
  HikarinagiPrice,
  HikarinagiSearchHit,
} from './types';

export const SITE_GALGAME_BASE = 'https://www.hikarinagi.org/galgames/';

export function normalizeHikarinagiId(input: unknown): string {
  if (input == null) return '';
  const raw = String(input).trim();
  if (!raw) return '';
  const urlMatch = raw.match(/hikarinagi\.org\/galgames?\/(\d+)/i);
  const candidate = urlMatch ? urlMatch[1] : raw.replace(/^#/, '');
  if (!/^\d+$/.test(candidate)) return '';
  const num = Number(candidate);
  if (!Number.isFinite(num) || num <= 0) return '';
  return String(Math.floor(num));
}

export function normalizeText(s: unknown): string {
  return String(s || '')
    .toLowerCase()
    .replace(/[\s　\-_~～·•・:：!！?？.,，。'"`「」『』【】()（）\[\]]+/g, '');
}

export function formatReleaseDate(detail: HikarinagiGalgameDetail | null | undefined): string {
  if (!detail) return '';
  if (detail.release_date_tbd) {
    return detail.release_date_tbd_note
      ? `待定（${detail.release_date_tbd_note}）`
      : '发售日待定';
  }
  const raw = String(detail.release_date || '').trim();
  if (!raw) return '';
  const datePart = raw.slice(0, 10);
  const precision = String(detail.release_date_precision || 'DAY').toUpperCase();
  if (precision === 'YEAR' && datePart.length >= 4) {
    return `${datePart.slice(0, 4)} 年`;
  }
  if (precision === 'MONTH' && datePart.length >= 7) {
    return `${datePart.slice(0, 4)} 年 ${Number(datePart.slice(5, 7))} 月`;
  }
  if (datePart.length === 10 && datePart[4] === '-' && datePart[7] === '-') {
    return `${datePart.slice(0, 4)} 年 ${Number(datePart.slice(5, 7))} 月 ${Number(datePart.slice(8, 10))} 日`;
  }
  return datePart;
}

export function formatStatusLabel(detail: HikarinagiGalgameDetail | null | undefined): string {
  if (!detail) return '';
  const st = String(detail.release_status || detail.dev_status || '').toUpperCase();
  switch (st) {
    case 'RELEASED':
      return '已发售';
    case 'PLANNED':
    case 'IN_DEVELOPMENT':
      return '开发中 / 预定发售';
    case 'CANCELLED':
      return '已取消';
    default:
      return st || '';
  }
}

export function formatLangLabel(lang: string | undefined): string {
  const code = String(lang || '').trim().toLowerCase();
  if (!code) return '';
  const map: Record<string, string> = {
    ja: '日语',
    jp: '日语',
    'ja-jp': '日语',
    zh: '中文',
    'zh-cn': '简体中文',
    'zh-hans': '简体中文',
    'zh-tw': '繁体中文',
    'zh-hant': '繁体中文',
    en: '英语',
    'en-us': '英语',
    ko: '韩语',
    'ko-kr': '韩语',
  };
  return map[code] || String(lang || '');
}

export function formatRelativeTime(tsMs: number): string {
  if (!tsMs) return '';
  const diffSec = Math.max(0, Math.floor((Date.now() - tsMs) / 1000));
  if (diffSec < 60) return '刚刚';
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`;
  return `${Math.floor(diffSec / 86400)} 天前`;
}

export function formatPrice(p: HikarinagiPrice | null | undefined): string {
  if (!p) return '';
  const parts: string[] = [];
  if (p.version) parts.push(String(p.version));
  const amount = p.amount != null ? Number(p.amount).toLocaleString() : '';
  const currency = p.currency ? String(p.currency) : '';
  const money = [amount, currency].filter(Boolean).join(' ');
  if (money) parts.push(money);
  if (p.tax_included === true) parts.push('含税');
  else if (p.tax_included === false) parts.push('不含税');
  return parts.join(' · ');
}

export function isMediaSensitive(item: HikarinagiMedia | null | undefined): boolean {
  if (!item) return false;
  return Number(item.sexual || 0) > 0 || Number(item.violence || 0) > 0;
}

export function pickBestLogo(
  logos: HikarinagiLogo[] | undefined,
  preferredLang: string,
  appLang: string
): HikarinagiLogo | null {
  if (!Array.isArray(logos) || logos.length === 0) return null;
  const valid = logos.filter((l) => l && l.url);
  if (valid.length === 0) return null;

  let order: string[];
  if (preferredLang === 'zh') {
    order = ['zh-hans', 'zh-cn', 'zh', 'zh-hant', 'zh-tw', 'ja', 'en'];
  } else if (preferredLang === 'ja') {
    order = ['ja', 'jp', 'zh-hans', 'zh-cn', 'zh', 'en'];
  } else if (preferredLang === 'en') {
    order = ['en', 'en-us', 'ja', 'zh-hans', 'zh'];
  } else {
    const langLower = String(appLang || 'zh-CN').toLowerCase();
    if (langLower.startsWith('ja')) {
      order = ['ja', 'jp', 'zh-hans', 'zh-cn', 'zh', 'en'];
    } else if (langLower.startsWith('en')) {
      order = ['en', 'en-us', 'ja', 'zh-hans', 'zh'];
    } else {
      order = ['zh-hans', 'zh-cn', 'zh', 'zh-hant', 'zh-tw', 'ja', 'en'];
    }
  }

  for (const target of order) {
    const hit = valid.find((l) => {
      const code = String(l.language || '').toLowerCase();
      return code === target || code.startsWith(`${target}-`);
    });
    if (hit) return hit;
  }
  return valid[0];
}

export function sortHeroImages(
  images: HikarinagiMedia[] | undefined,
  blurNsfw: boolean
): HikarinagiMedia[] {
  if (!Array.isArray(images)) return [];
  const valid = images.filter((img) => img && img.url);
  const scoreImage = (img: HikarinagiMedia): number => {
    const w = Number(img.width || 0);
    const h = Number(img.height || 0);
    const ratio = w > 0 && h > 0 ? w / h : 1.77;
    const isLandscape = ratio >= 1.25;
    const sensitive = isMediaSensitive(img);
    let score = 0;
    if (isLandscape) score += 10000;
    if (!sensitive) score += blurNsfw ? 50000 : 5000;
    else if (blurNsfw) score -= 20000;
    score -= Math.abs(ratio - 1.78) * 1200;
    score += Math.min(w * h, 2560 * 1440) / 1000;
    return score;
  };
  return valid.slice().sort((a, b) => scoreImage(b) - scoreImage(a));
}

export function pickBestBanner(
  detail: HikarinagiGalgameDetail | null | undefined,
  game: Game | undefined,
  blurNsfw: boolean
): BannerPick {
  const sorted = sortHeroImages(detail?.images, blurNsfw);
  if (sorted.length > 0) {
    const best = sorted[0];
    return {
      url: best.url,
      sensitive: isMediaSensitive(best),
      fromCover: false,
    };
  }

  const covers = Array.isArray(detail?.covers) ? detail!.covers.filter((c) => c && c.url) : [];
  if (covers.length > 0) {
    const safeCover = blurNsfw ? covers.find((c) => !isMediaSensitive(c)) : undefined;
    const picked = safeCover || covers[0];
    return {
      url: picked.url,
      sensitive: isMediaSensitive(picked) || Boolean(detail?.nsfw),
      fromCover: true,
    };
  }

  const fallback = game?.cover_url || game?.cover_source_url || '';
  return {
    url: fallback,
    sensitive: Boolean(game?.is_nsfw),
    fromCover: true,
  };
}

export function pickBestSearchHit(
  items: HikarinagiSearchHit[] | undefined,
  game: Game | undefined
): HikarinagiSearchHit | null {
  const hits = (Array.isArray(items) ? items : []).filter(
    (item) => item && item.type === 'galgame' && Number(item.id) > 0
  );
  if (hits.length === 0) return null;

  const targetNames = [game?.name, ...(Array.isArray(game?.aliases) ? game!.aliases : [])]
    .map(normalizeText)
    .filter(Boolean);

  for (const h of hits) {
    const titleNorm = normalizeText(h.title);
    const subNorm = normalizeText(h.subtitle);
    for (const target of targetNames) {
      if (target && (titleNorm === target || subNorm === target)) {
        return h;
      }
    }
  }

  const companyNorm = normalizeText(game?.company);
  if (companyNorm) {
    const byCompany = hits.find((h) => {
      const devNorm = normalizeText(h.developer);
      return (
        Boolean(devNorm) &&
        (devNorm === companyNorm || devNorm.includes(companyNorm) || companyNorm.includes(devNorm))
      );
    });
    if (byCompany) return byCompany;
  }

  return hits[0];
}

export function buildBangumiUrl(raw: string | number | undefined): string {
  const v = String(raw ?? '').trim();
  if (!v) return '';
  if (/^https?:\/\//i.test(v)) return v;
  const digits = v.replace(/[^\d]/g, '');
  return digits ? `https://bgm.tv/subject/${digits}` : '';
}

export function buildVndbUrl(raw: string | undefined): string {
  const v = String(raw ?? '').trim();
  if (!v) return '';
  if (/^https?:\/\//i.test(v)) return v;
  const normalized = /^v\d+$/i.test(v) ? v.toLowerCase() : /^\d+$/.test(v) ? `v${v}` : '';
  return normalized ? `https://vndb.org/${normalized}` : '';
}

export function getPrimaryAndSubTitle(
  detail: HikarinagiGalgameDetail | null | undefined,
  game: Game | undefined
): { title: string; subtitle: string } {
  const localName = String(game?.name || '').trim();
  const trans = String(detail?.trans_title || '').trim();
  const origin = String(detail?.origin_title || '').trim();
  const en = String(detail?.en_title || '').trim();

  const title = trans || localName || origin || en || '';
  let subtitle = '';
  if (origin && origin !== title) {
    subtitle = origin;
  } else if (en && en !== title) {
    subtitle = en;
  } else if (localName && localName !== title) {
    subtitle = localName;
  }
  return { title, subtitle };
}
