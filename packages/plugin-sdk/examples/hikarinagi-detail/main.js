// hikarinagi-detail/src/format.ts
var SITE_GALGAME_BASE = "https://www.hikarinagi.org/galgames/";
function normalizeHikarinagiId(input) {
  if (input == null) return "";
  const raw = String(input).trim();
  if (!raw) return "";
  const urlMatch = raw.match(/hikarinagi\.org\/galgames?\/(\d+)/i);
  const candidate = urlMatch ? urlMatch[1] : raw.replace(/^#/, "");
  if (!/^\d+$/.test(candidate)) return "";
  const num = Number(candidate);
  if (!Number.isFinite(num) || num <= 0) return "";
  return String(Math.floor(num));
}
function normalizeText(s) {
  return String(s || "").toLowerCase().replace(/[\s　\-_~～·•・:：!！?？.,，。'"`「」『』【】()（）\[\]]+/g, "");
}
function formatReleaseDate(detail) {
  if (!detail) return "";
  if (detail.release_date_tbd) {
    return detail.release_date_tbd_note ? `\u5F85\u5B9A\uFF08${detail.release_date_tbd_note}\uFF09` : "\u53D1\u552E\u65E5\u5F85\u5B9A";
  }
  const raw = String(detail.release_date || "").trim();
  if (!raw) return "";
  const datePart = raw.slice(0, 10);
  const precision = String(detail.release_date_precision || "DAY").toUpperCase();
  if (precision === "YEAR" && datePart.length >= 4) {
    return `${datePart.slice(0, 4)} \u5E74`;
  }
  if (precision === "MONTH" && datePart.length >= 7) {
    return `${datePart.slice(0, 4)} \u5E74 ${Number(datePart.slice(5, 7))} \u6708`;
  }
  if (datePart.length === 10 && datePart[4] === "-" && datePart[7] === "-") {
    return `${datePart.slice(0, 4)} \u5E74 ${Number(datePart.slice(5, 7))} \u6708 ${Number(datePart.slice(8, 10))} \u65E5`;
  }
  return datePart;
}
function formatStatusLabel(detail) {
  if (!detail) return "";
  const st = String(detail.release_status || detail.dev_status || "").toUpperCase();
  switch (st) {
    case "RELEASED":
      return "\u5DF2\u53D1\u552E";
    case "PLANNED":
    case "IN_DEVELOPMENT":
      return "\u5F00\u53D1\u4E2D / \u9884\u5B9A\u53D1\u552E";
    case "CANCELLED":
      return "\u5DF2\u53D6\u6D88";
    default:
      return st || "";
  }
}
function formatLangLabel(lang) {
  const code = String(lang || "").trim().toLowerCase();
  if (!code) return "";
  const map = {
    ja: "\u65E5\u8BED",
    jp: "\u65E5\u8BED",
    "ja-jp": "\u65E5\u8BED",
    zh: "\u4E2D\u6587",
    "zh-cn": "\u7B80\u4F53\u4E2D\u6587",
    "zh-hans": "\u7B80\u4F53\u4E2D\u6587",
    "zh-tw": "\u7E41\u4F53\u4E2D\u6587",
    "zh-hant": "\u7E41\u4F53\u4E2D\u6587",
    en: "\u82F1\u8BED",
    "en-us": "\u82F1\u8BED",
    ko: "\u97E9\u8BED",
    "ko-kr": "\u97E9\u8BED"
  };
  return map[code] || String(lang || "");
}
function formatPrice(p) {
  if (!p) return "";
  const parts = [];
  if (p.version) parts.push(String(p.version));
  const amount = p.amount != null ? Number(p.amount).toLocaleString() : "";
  const currency = p.currency ? String(p.currency) : "";
  const money = [amount, currency].filter(Boolean).join(" ");
  if (money) parts.push(money);
  if (p.tax_included === true) parts.push("\u542B\u7A0E");
  else if (p.tax_included === false) parts.push("\u4E0D\u542B\u7A0E");
  return parts.join(" \xB7 ");
}
function isMediaSensitive(item) {
  if (!item) return false;
  return Number(item.sexual || 0) > 0 || Number(item.violence || 0) > 0;
}
function pickBestLogo(logos, preferredLang, appLang) {
  if (!Array.isArray(logos) || logos.length === 0) return null;
  const valid = logos.filter((l) => l && l.url);
  if (valid.length === 0) return null;
  let order;
  if (preferredLang === "zh") {
    order = ["zh-hans", "zh-cn", "zh", "zh-hant", "zh-tw", "ja", "en"];
  } else if (preferredLang === "ja") {
    order = ["ja", "jp", "zh-hans", "zh-cn", "zh", "en"];
  } else if (preferredLang === "en") {
    order = ["en", "en-us", "ja", "zh-hans", "zh"];
  } else {
    const langLower = String(appLang || "zh-CN").toLowerCase();
    if (langLower.startsWith("ja")) {
      order = ["ja", "jp", "zh-hans", "zh-cn", "zh", "en"];
    } else if (langLower.startsWith("en")) {
      order = ["en", "en-us", "ja", "zh-hans", "zh"];
    } else {
      order = ["zh-hans", "zh-cn", "zh", "zh-hant", "zh-tw", "ja", "en"];
    }
  }
  for (const target of order) {
    const hit = valid.find((l) => {
      const code = String(l.language || "").toLowerCase();
      return code === target || code.startsWith(`${target}-`);
    });
    if (hit) return hit;
  }
  return valid[0];
}
function sortHeroImages(images, blurNsfw) {
  if (!Array.isArray(images)) return [];
  const valid = images.filter((img) => img && img.url);
  const scoreImage = (img) => {
    const w = Number(img.width || 0);
    const h = Number(img.height || 0);
    const ratio = w > 0 && h > 0 ? w / h : 1.77;
    const isLandscape = ratio >= 1.25;
    const sensitive = isMediaSensitive(img);
    let score = 0;
    if (isLandscape) score += 1e4;
    if (!sensitive) score += blurNsfw ? 5e4 : 5e3;
    else if (blurNsfw) score -= 2e4;
    score -= Math.abs(ratio - 1.78) * 1200;
    score += Math.min(w * h, 2560 * 1440) / 1e3;
    return score;
  };
  return valid.slice().sort((a, b) => scoreImage(b) - scoreImage(a));
}
function pickBestBanner(detail, game, blurNsfw) {
  const sorted = sortHeroImages(detail?.images, blurNsfw);
  if (sorted.length > 0) {
    const best = sorted[0];
    return {
      url: best.url,
      sensitive: isMediaSensitive(best),
      fromCover: false
    };
  }
  const covers = Array.isArray(detail?.covers) ? detail.covers.filter((c) => c && c.url) : [];
  if (covers.length > 0) {
    const safeCover = blurNsfw ? covers.find((c) => !isMediaSensitive(c)) : void 0;
    const picked = safeCover || covers[0];
    return {
      url: picked.url,
      sensitive: isMediaSensitive(picked) || Boolean(detail?.nsfw),
      fromCover: true
    };
  }
  const fallback = game?.cover_url || game?.cover_source_url || "";
  return {
    url: fallback,
    sensitive: Boolean(game?.is_nsfw),
    fromCover: true
  };
}
function pickBestSearchHit(items, game) {
  const hits = (Array.isArray(items) ? items : []).filter(
    (item) => item && item.type === "galgame" && Number(item.id) > 0
  );
  if (hits.length === 0) return null;
  const targetNames = [game?.name, ...Array.isArray(game?.aliases) ? game.aliases : []].map(normalizeText).filter(Boolean);
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
      return Boolean(devNorm) && (devNorm === companyNorm || devNorm.includes(companyNorm) || companyNorm.includes(devNorm));
    });
    if (byCompany) return byCompany;
  }
  return hits[0];
}
function buildBangumiUrl(raw) {
  const v = String(raw ?? "").trim();
  if (!v) return "";
  if (/^https?:\/\//i.test(v)) return v;
  const digits = v.replace(/[^\d]/g, "");
  return digits ? `https://bgm.tv/subject/${digits}` : "";
}
function buildVndbUrl(raw) {
  const v = String(raw ?? "").trim();
  if (!v) return "";
  if (/^https?:\/\//i.test(v)) return v;
  const normalized = /^v\d+$/i.test(v) ? v.toLowerCase() : /^\d+$/.test(v) ? `v${v}` : "";
  return normalized ? `https://vndb.org/${normalized}` : "";
}
function getPrimaryAndSubTitle(detail, game) {
  const localName = String(game?.name || "").trim();
  const trans = String(detail?.trans_title || "").trim();
  const origin = String(detail?.origin_title || "").trim();
  const en = String(detail?.en_title || "").trim();
  const title = trans || localName || origin || en || "";
  let subtitle = "";
  if (origin && origin !== title) {
    subtitle = origin;
  } else if (en && en !== title) {
    subtitle = en;
  } else if (localName && localName !== title) {
    subtitle = localName;
  }
  return { title, subtitle };
}

// hikarinagi-detail/src/api.ts
var API_BASE = "https://api.hikarinagi.org/v3";
function createHikarinagiApi(ctx) {
  async function resolveToken() {
    const cfg = ctx.config.get() || {};
    const custom = String(cfg.accessToken || "").trim();
    if (custom) return custom;
    const token = await ctx.metadata.getAuthToken("hikarinagi");
    return String(token || "").trim();
  }
  async function apiGet(path) {
    const token = await resolveToken();
    if (!token) {
      throw new Error("\u672A\u83B7\u53D6\u5230 Hikarinagi \u8BBF\u95EE\u4EE4\u724C\uFF0C\u8BF7\u5148\u5728\u8BBE\u7F6E\u4E2D\u767B\u5F55 Hikarinagi \u6216\u586B\u5199\u81EA\u5B9A\u4E49 Token");
    }
    const resp = await fetch(`${API_BASE}${path}`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json"
      }
    });
    const text = await resp.text();
    let json = null;
    try {
      json = text ? JSON.parse(text) : null;
    } catch {
      json = null;
    }
    if (!resp.ok) {
      const msg = json?.message || json?.error?.message || `HTTP ${resp.status}`;
      throw new Error(`Hikarinagi API \u8BF7\u6C42\u5931\u8D25\uFF1A${msg}`);
    }
    if (!json || json.success === false) {
      const msg = json?.message || "\u63A5\u53E3\u8FD4\u56DE\u5F02\u5E38";
      throw new Error(`Hikarinagi API \u9519\u8BEF\uFF1A${msg}`);
    }
    return json.data;
  }
  async function fetchGalgameDetail(hikarinagiId) {
    const data = await apiGet(
      `/galgames/${encodeURIComponent(hikarinagiId)}`
    );
    if (!data || !data.id) {
      throw new Error(`\u672A\u627E\u5230 Hikarinagi \u6761\u76EE #${hikarinagiId}`);
    }
    return data;
  }
  async function searchGalgameId(game) {
    const queries = [];
    if (game?.name && String(game.name).trim()) {
      queries.push(String(game.name).trim());
    }
    if (Array.isArray(game?.aliases)) {
      for (const raw of game.aliases) {
        if (queries.length >= 3) break;
        const alias = String(raw || "").trim();
        if (alias && !queries.includes(alias)) {
          queries.push(alias);
        }
      }
    }
    for (const q of queries) {
      const data = await apiGet(
        `/search?q=${encodeURIComponent(q)}&types=galgame&page=1&page_size=5`
      );
      const hit = pickBestSearchHit(data?.items, game);
      if (hit && Number(hit.id) > 0) {
        return {
          hikarinagiId: String(hit.id),
          matchedTitle: String(hit.title || "")
        };
      }
    }
    return null;
  }
  return {
    fetchGalgameDetail,
    searchGalgameId
  };
}

// hikarinagi-detail/src/components/DetailAside.tsx
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
  useState
} from "lunabox";
import { Fragment, jsx, jsxs } from "lunabox/jsx-runtime";
var ALIAS_PREVIEW = 3;
function InfoRow({
  label,
  value,
  children
}) {
  const v = String(value || "").trim();
  if (!v && !children) return null;
  return /* @__PURE__ */ jsxs(Row, { gap: "md", align: "start", width: "full", children: [
    /* @__PURE__ */ jsx(Box, { width: 44, shrink: 0, children: /* @__PURE__ */ jsx(Text, { text: label, size: "sm", tone: "muted" }) }),
    /* @__PURE__ */ jsx(Column, { grow: 1, basis: 0, minWidth: 0, gap: 4, children: children ? children : /* @__PURE__ */ jsx(Text, { text: v, size: "sm", tone: "default", selectable: true }) })
  ] });
}
function AccentLink({ k, text, onPress }) {
  return /* @__PURE__ */ jsx(Box, { k, alignSelf: "start", onPress, label: text, children: /* @__PURE__ */ jsx(Text, { text, size: "sm", tone: "accent" }) });
}
function createDetailAside(ctx, store) {
  return function DetailAside({ game }) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : "";
    const [aliasesOpen, setAliasesOpen] = useState(false);
    useEffect(() => {
      setAliasesOpen(false);
    }, [gameId]);
    if (state.status === "loading" && !detail) {
      return /* @__PURE__ */ jsxs(Column, { k: "aside-loading", gap: "md", width: "full", children: [
        /* @__PURE__ */ jsxs(Row, { justify: "between", align: "center", width: "full", children: [
          /* @__PURE__ */ jsx(Text, { text: "\u6E38\u620F\u8BE6\u60C5", size: "base", weight: "semibold", tone: "strong" }),
          /* @__PURE__ */ jsx(Text, { text: "\u540C\u6B65\u4E2D\u2026", size: "xs", tone: "muted" })
        ] }),
        /* @__PURE__ */ jsx(Skeleton, { lines: 7, height: 14 })
      ] });
    }
    const links = [];
    if (detail?.id) {
      links.push({
        label: "Hikarinagi",
        url: `${SITE_GALGAME_BASE}${detail.id}`,
        icon: "open-in-new"
      });
    }
    if (Array.isArray(detail?.steam_apps)) {
      detail.steam_apps.forEach((sa, idx) => {
        if (sa && Number(sa.app_id) > 0) {
          links.push({
            label: idx === 0 ? "Steam" : `Steam #${idx + 1}`,
            url: `https://store.steampowered.com/app/${sa.app_id}/`,
            icon: "steam"
          });
        }
      });
    }
    if (detail?.external_source) {
      const bgmUrl = buildBangumiUrl(detail.external_source.bangumi);
      if (bgmUrl) links.push({ label: "Bangumi", url: bgmUrl, icon: "open-in-new" });
      const vndbUrl = buildVndbUrl(detail.external_source.vndb);
      if (vndbUrl) links.push({ label: "VNDB", url: vndbUrl, icon: "open-in-new" });
    }
    if (Array.isArray(detail?.external_links)) {
      detail.external_links.forEach((el) => {
        if (el?.url) {
          links.push({
            label: String(el.label || el.name || "\u5916\u90E8\u94FE\u63A5"),
            url: String(el.url),
            icon: "link-variant"
          });
        }
      });
    }
    const platforms = Array.isArray(detail?.platforms) ? detail.platforms.map((p) => String(p || "").trim()).filter(Boolean) : [];
    const prices = Array.isArray(detail?.prices) ? detail.prices.map(formatPrice).filter(Boolean) : [];
    const langs = [];
    if (Array.isArray(detail?.languages) && detail.languages.length > 0) {
      for (const l of detail.languages) {
        const lbl = formatLangLabel(l);
        if (lbl && !langs.includes(lbl)) langs.push(lbl);
      }
    } else if (detail?.origin_lang) {
      const lbl = formatLangLabel(detail.origin_lang);
      if (lbl) langs.push(lbl);
    }
    const homepage = String(detail?.homepage || "").trim();
    const aliases = Array.isArray(detail?.aliases) ? detail.aliases.map((a) => String(a || "").trim()).filter(Boolean) : [];
    const shownAliases = aliasesOpen ? aliases : aliases.slice(0, ALIAS_PREVIEW);
    return /* @__PURE__ */ jsxs(Column, { k: `aside-root-${gameId}`, gap: 20, width: "full", children: [
      /* @__PURE__ */ jsxs(Column, { k: "aside-info-sec", gap: "md", width: "full", children: [
        /* @__PURE__ */ jsxs(Row, { justify: "between", align: "center", width: "full", children: [
          /* @__PURE__ */ jsxs(Row, { gap: "xs", align: "center", children: [
            /* @__PURE__ */ jsx(Text, { text: "\u6E38\u620F\u4FE1\u606F", size: "sm", weight: "semibold", tone: "strong" }),
            detail?.nsfw ? /* @__PURE__ */ jsx(Badge, { label: "R-18", tone: "danger" }) : null
          ] }),
          detail ? /* @__PURE__ */ jsxs(Row, { gap: "xs", align: "center", children: [
            /* @__PURE__ */ jsx(
              IconButton,
              {
                k: "aside-refresh-btn",
                icon: "refresh",
                size: "sm",
                label: "\u5237\u65B0 Hikarinagi \u8BE6\u60C5",
                disabled: state.refreshing,
                onPress: () => store.loadGame(game, { force: true })
              }
            ),
            /* @__PURE__ */ jsx(
              IconButton,
              {
                k: "aside-open-hn",
                icon: "open-in-new",
                size: "sm",
                label: "\u5728\u6D4F\u89C8\u5668\u6253\u5F00 Hikarinagi",
                onPress: () => ctx.shell.openURL(`${SITE_GALGAME_BASE}${detail.id}`)
              }
            )
          ] }) : /* @__PURE__ */ jsxs(Row, { gap: "xs", align: "center", children: [
            /* @__PURE__ */ jsx(
              Text,
              {
                text: state.status === "error" ? "\u540C\u6B65\u5931\u8D25" : "\u672A\u5339\u914D",
                size: "xs",
                tone: "warning"
              }
            ),
            /* @__PURE__ */ jsx(
              IconButton,
              {
                k: "aside-retry-btn",
                icon: "refresh",
                size: "sm",
                label: "\u91CD\u8BD5\u5339\u914D",
                disabled: state.refreshing,
                onPress: () => store.loadGame(game, { force: true })
              }
            )
          ] })
        ] }),
        detail ? /* @__PURE__ */ jsxs(Column, { k: "aside-kv-list", gap: 12, width: "full", children: [
          /* @__PURE__ */ jsx(InfoRow, { label: "\u5F00\u53D1", value: detail.developer }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u53D1\u884C", value: detail.publisher }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u9996\u53D1", value: formatReleaseDate(detail) }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u7C7B\u578B", value: detail.adv_type }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u5E73\u53F0", value: platforms.join("\u3001") }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u8BED\u8A00", value: langs.join("\u3001") }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u5F15\u64CE", value: detail.engine }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u72B6\u6001", value: formatStatusLabel(detail) }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u5206\u7EA7", value: detail.nsfw ? "R-18 / \u6210\u4EBA\u5411" : "\u5168\u5E74\u9F84" }),
          prices.length > 0 ? /* @__PURE__ */ jsx(InfoRow, { label: "\u5B9A\u4EF7", value: prices.slice(0, 3).join("\n") }) : null,
          homepage ? /* @__PURE__ */ jsx(InfoRow, { label: "\u5B98\u7F51", children: /* @__PURE__ */ jsx(
            AccentLink,
            {
              k: "aside-homepage",
              text: homepage,
              onPress: () => ctx.shell.openURL(homepage)
            }
          ) }) : null,
          /* @__PURE__ */ jsx(InfoRow, { label: "\u539F\u540D", value: detail.origin_title }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u8BD1\u540D", value: detail.trans_title }),
          /* @__PURE__ */ jsx(InfoRow, { label: "\u82F1\u6587", value: detail.en_title }),
          aliases.length > 0 ? /* @__PURE__ */ jsxs(InfoRow, { label: "\u522B\u540D", children: [
            shownAliases.map((al, idx) => /* @__PURE__ */ jsx(
              Text,
              {
                text: al,
                size: "sm",
                tone: "default",
                selectable: true
              },
              `aside-alias-${idx}`
            )),
            aliases.length > ALIAS_PREVIEW ? /* @__PURE__ */ jsx(
              AccentLink,
              {
                k: "aside-alias-toggle",
                text: aliasesOpen ? "\u6536\u8D77" : `\u5C55\u5F00\uFF08\u5171 ${aliases.length} \u4E2A\uFF09`,
                onPress: () => setAliasesOpen(!aliasesOpen)
              }
            ) : null
          ] }) : null
        ] }) : /* @__PURE__ */ jsxs(Column, { gap: "sm", width: "full", children: [
          /* @__PURE__ */ jsx(
            Text,
            {
              text: state.error || "\u5C1A\u672A\u81EA\u52A8\u5339\u914D\u5230 Hikarinagi \u6761\u76EE\u3002",
              size: "xs",
              tone: state.status === "error" ? "danger" : "muted"
            }
          ),
          game?.company ? /* @__PURE__ */ jsx(InfoRow, { label: "\u5F00\u53D1", value: game.company }) : null,
          game?.release_date ? /* @__PURE__ */ jsx(InfoRow, { label: "\u9996\u53D1", value: game.release_date }) : null
        ] })
      ] }),
      links.length > 0 ? /* @__PURE__ */ jsxs(Fragment, { children: [
        /* @__PURE__ */ jsx(Divider, { subtle: true }),
        /* @__PURE__ */ jsxs(Column, { k: "aside-links-sec", gap: "sm", width: "full", children: [
          /* @__PURE__ */ jsx(Text, { text: "\u76F8\u5173\u94FE\u63A5", size: "sm", weight: "semibold", tone: "strong" }),
          /* @__PURE__ */ jsx(Row, { gap: 6, wrap: true, width: "full", children: links.map((lk, idx) => /* @__PURE__ */ jsxs(
            Row,
            {
              gap: 6,
              align: "center",
              paddingX: 10,
              paddingY: 5,
              radius: "md",
              background: "cardBg",
              border: 1,
              borderColor: "cardBorder",
              hoverBackground: "accentSoft",
              label: lk.label,
              onPress: () => ctx.shell.openURL(lk.url),
              children: [
                /* @__PURE__ */ jsx(Icon, { name: lk.icon, size: 14, tone: "accent" }),
                /* @__PURE__ */ jsx(Text, { text: lk.label, size: "xs", weight: "medium", tone: "default", wrap: false })
              ]
            },
            `ext-link-${idx}`
          )) })
        ] })
      ] }) : null
    ] });
  };
}

// hikarinagi-detail/src/components/HeroSection.tsx
import { HeroBanner, useEffect as useEffect2, useRef, useState as useState2 } from "lunabox";
import { jsx as jsx2 } from "lunabox/jsx-runtime";
function createHeroSection(ctx, store) {
  return function HeroSection({ game, blurNsfw, contentMaxWidth }) {
    const state = store.useGameDetail(game);
    const cfg = store.getConfig();
    const detail = state.detail;
    const shouldBlurNsfw = Boolean(blurNsfw);
    const gameId = game?.id ? String(game.id) : "";
    const [selectedIdx, setSelectedIdx] = useState2(0);
    const prevBannerRef = useRef("");
    useEffect2(() => {
      setSelectedIdx(0);
      prevBannerRef.current = "";
    }, [gameId]);
    const sortedImages = sortHeroImages(detail?.images, shouldBlurNsfw);
    const defaultBanner = pickBestBanner(detail, game, shouldBlurNsfw);
    const totalImages = sortedImages.length;
    const safeIdx = totalImages > 0 ? (selectedIdx % totalImages + totalImages) % totalImages : 0;
    const activeImg = totalImages > 0 ? sortedImages[safeIdx] : null;
    const activeBannerUrl = activeImg ? activeImg.url : defaultBanner.url;
    const activeBannerSensitive = activeImg ? isMediaSensitive(activeImg) : defaultBanner.sensitive;
    const fallbackBanner = prevBannerRef.current && prevBannerRef.current !== activeBannerUrl ? prevBannerRef.current : game?.cover_url || game?.cover_source_url || "";
    useEffect2(() => {
      if (activeBannerUrl) {
        prevBannerRef.current = activeBannerUrl;
      }
    }, [activeBannerUrl]);
    const logo = detail ? pickBestLogo(detail.logos, cfg.preferredLogoLang || "auto", ctx.i18n.getLanguage()) : null;
    const { title, subtitle } = getPrimaryAndSubTitle(detail, game);
    const meta = [];
    const dev = detail?.developer || game?.company || "";
    if (dev) {
      meta.push({ icon: "palette", text: dev });
    }
    const relDate = detail ? formatReleaseDate(detail) : game?.release_date || "";
    if (relDate) {
      meta.push({ icon: "calendar-month-outline", text: relDate });
    }
    const platforms = Array.isArray(detail?.platforms) ? detail.platforms.map((p) => String(p || "").trim()).filter(Boolean) : [];
    if (platforms.length > 0) {
      meta.push({ icon: "gamepad-variant-outline", text: platforms.slice(0, 3).join("\u3001") });
    }
    const rawLangs = [];
    if (Array.isArray(detail?.languages) && detail.languages.length > 0) {
      for (const l of detail.languages) {
        const lbl = formatLangLabel(l);
        if (lbl && !rawLangs.includes(lbl)) rawLangs.push(lbl);
      }
    } else if (detail?.origin_lang) {
      const lbl = formatLangLabel(detail.origin_lang);
      if (lbl) rawLangs.push(lbl);
    }
    if (rawLangs.length > 0) {
      meta.push({ icon: "earth", text: rawLangs.slice(0, 4).join("\u3001") });
    } else if (detail?.adv_type) {
      meta.push({ icon: "tag-outline", text: String(detail.adv_type) });
    }
    const hnScore = Number(detail?.rating?.score || 0);
    const localScore = Number(game?.rating || 0);
    const rating = hnScore > 0 ? {
      score: hnScore,
      total: Number(detail?.rating?.total || 0),
      played: Number(detail?.rating?.played || 0)
    } : localScore > 0 ? { score: localScore, total: 0, played: 0 } : void 0;
    const rawHeight = Number(cfg.heroHeight);
    const heroHeight = Number.isFinite(rawHeight) && rawHeight >= 360 ? Math.min(760, rawHeight) : 560;
    const shouldBlurHero = Boolean(cfg.blurHeroBanner) || shouldBlurNsfw && activeBannerSensitive;
    const canPage = totalImages > 1;
    return /* @__PURE__ */ jsx2(
      HeroBanner,
      {
        k: `hero-${gameId}`,
        height: heroHeight,
        contentMaxWidth: contentMaxWidth || 1200,
        banner: activeBannerUrl,
        fallbackBanner,
        logo: logo?.url || "",
        title,
        subtitle,
        meta,
        rating,
        blur: shouldBlurHero,
        backSlot: "back",
        coverSlot: "cover",
        toolbarSlot: "toolbar",
        pager: canPage ? { current: safeIdx + 1, total: totalImages } : void 0,
        onPrev: canPage ? () => {
          setSelectedIdx((prev) => (prev - 1 + totalImages) % totalImages);
        } : void 0,
        onNext: canPage ? () => {
          setSelectedIdx((prev) => (prev + 1) % totalImages);
        } : void 0
      }
    );
  };
}

// hikarinagi-detail/src/components/OverviewTab.tsx
import { Box as Box2, Column as Column2, Divider as Divider2, NativeSlot, Row as Row2, Skeleton as Skeleton2, Text as Text2, useEffect as useEffect3, useState as useState3 } from "lunabox";
import { jsx as jsx3, jsxs as jsxs2 } from "lunabox/jsx-runtime";
var INTRO_PREVIEW = 220;
function Segmented({
  k,
  value,
  items,
  onChange
}) {
  return /* @__PURE__ */ jsx3(Row2, { k, gap: 2, padding: 2, radius: "md", background: "chipBg", align: "center", children: items.map((it) => {
    const active = it.key === value;
    return /* @__PURE__ */ jsx3(
      Box2,
      {
        paddingX: 10,
        paddingY: 3,
        radius: 6,
        background: active ? "cardBg" : "transparent",
        border: active ? 1 : 0,
        borderColor: "cardBorder",
        label: it.label,
        onPress: () => onChange(it.key),
        children: /* @__PURE__ */ jsx3(
          Text2,
          {
            text: it.label,
            size: "xs",
            weight: active ? "semibold" : "medium",
            tone: active ? "accent" : "muted"
          }
        )
      },
      `${k}-${it.key}`
    );
  }) });
}
function createOverviewTab(store) {
  return function OverviewTab({ game }) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : "";
    const [introLang, setIntroLang] = useState3("zh");
    const [introOpen, setIntroOpen] = useState3(false);
    useEffect3(() => {
      setIntroLang("zh");
      setIntroOpen(false);
    }, [gameId]);
    const transIntro = String(detail?.trans_intro || "").trim();
    const originIntro = String(detail?.origin_intro || "").trim();
    const hasBothIntros = Boolean(transIntro && originIntro && transIntro !== originIntro);
    const activeIntro = introLang === "origin" ? originIntro || transIntro : transIntro || originIntro;
    const introChars = Array.from(activeIntro);
    const collapsible = introChars.length > INTRO_PREVIEW;
    const shownIntro = collapsible && !introOpen ? `${introChars.slice(0, INTRO_PREVIEW).join("").trimEnd()}\u2026` : activeIntro;
    const loading = state.status === "loading" && !detail && !game?.summary;
    return /* @__PURE__ */ jsxs2(Column2, { k: `overview-${gameId}`, gap: 24, width: "full", children: [
      loading || activeIntro ? /* @__PURE__ */ jsxs2(Column2, { k: "ov-intro-sec", gap: "md", width: "full", children: [
        /* @__PURE__ */ jsxs2(Row2, { justify: "between", align: "center", width: "full", children: [
          /* @__PURE__ */ jsx3(Text2, { text: "\u7B80\u4ECB", size: "lg", weight: "semibold", tone: "strong" }),
          hasBothIntros ? /* @__PURE__ */ jsx3(
            Segmented,
            {
              k: "ov-lang",
              value: introLang,
              items: [
                { key: "zh", label: "\u4E2D\u6587" },
                { key: "origin", label: "\u539F\u6587" }
              ],
              onChange: (v) => {
                setIntroLang(v);
                setIntroOpen(false);
              }
            }
          ) : null
        ] }),
        loading ? /* @__PURE__ */ jsx3(Skeleton2, { lines: 4, height: 14 }) : /* @__PURE__ */ jsxs2(Column2, { gap: "sm", width: "full", children: [
          /* @__PURE__ */ jsx3(Text2, { text: shownIntro, size: "sm", tone: "default", selectable: true }),
          collapsible ? /* @__PURE__ */ jsx3(
            Box2,
            {
              k: "ov-intro-toggle",
              alignSelf: "start",
              label: introOpen ? "\u6536\u8D77" : "\u5C55\u5F00",
              onPress: () => setIntroOpen(!introOpen),
              children: /* @__PURE__ */ jsx3(Text2, { text: introOpen ? "\u6536\u8D77" : "\u5C55\u5F00", size: "sm", tone: "accent" })
            }
          ) : null
        ] })
      ] }) : /* @__PURE__ */ jsx3(NativeSlot, { k: "ov-native-summary", name: "summary" }),
      /* @__PURE__ */ jsx3(Divider2, { subtle: true }),
      /* @__PURE__ */ jsxs2(Column2, { k: "ov-tags-sec", gap: "md", width: "full", children: [
        /* @__PURE__ */ jsx3(Text2, { text: "\u6807\u7B7E", size: "lg", weight: "semibold", tone: "strong" }),
        /* @__PURE__ */ jsx3(NativeSlot, { k: "ov-native-tags", name: "tags" })
      ] })
    ] });
  };
}

// hikarinagi-detail/src/components/ScreenshotsTab.tsx
import {
  Button,
  Column as Column3,
  Empty,
  Grid,
  Image,
  Row as Row3,
  Skeleton as Skeleton3,
  Text as Text3,
  useEffect as useEffect4,
  useState as useState4
} from "lunabox";
import { jsx as jsx4, jsxs as jsxs3 } from "lunabox/jsx-runtime";
function createScreenshotsTab(store) {
  return function ScreenshotsTab({ game, blurNsfw }) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const shouldBlurNsfw = Boolean(blurNsfw);
    const gameId = game?.id ? String(game.id) : "";
    const [revealNsfw, setRevealNsfw] = useState4(false);
    const [revealedMap, setRevealedMap] = useState4({});
    useEffect4(() => {
      setRevealNsfw(false);
      setRevealedMap({});
    }, [gameId]);
    if (state.status === "loading" && !detail) {
      return /* @__PURE__ */ jsxs3(Column3, { k: "ss-loading", gap: "md", width: "full", children: [
        /* @__PURE__ */ jsx4(Text3, { text: "\u6B63\u5728\u52A0\u8F7D\u6E38\u620F\u622A\u56FE\u2026", size: "sm", tone: "muted" }),
        /* @__PURE__ */ jsx4(Skeleton3, { lines: 5, height: 18 })
      ] });
    }
    if (!detail) {
      return /* @__PURE__ */ jsxs3(Column3, { k: "ss-unmatched", width: "full", align: "center", gap: "sm", children: [
        /* @__PURE__ */ jsx4(
          Empty,
          {
            icon: "image-off",
            title: state.status === "error" ? "\u83B7\u53D6 Hikarinagi \u622A\u56FE\u5931\u8D25" : "\u672A\u5339\u914D\u5230 Hikarinagi \u6761\u76EE",
            description: state.error || "\u5C1A\u672A\u5339\u914D\u5230\u5BF9\u5E94\u7684 Hikarinagi \u6761\u76EE\uFF0C\u8BF7\u70B9\u51FB\u91CD\u8BD5\u3002"
          }
        ),
        /* @__PURE__ */ jsx4(
          Button,
          {
            k: "ss-retry",
            label: "\u91CD\u65B0\u5339\u914D",
            icon: "refresh",
            variant: "secondary",
            size: "sm",
            onPress: () => store.loadGame(game, { force: true })
          }
        )
      ] });
    }
    const images = Array.isArray(detail.images) ? detail.images.filter((i) => i && i.url) : [];
    const sensitiveCount = images.filter(isMediaSensitive).length;
    return /* @__PURE__ */ jsxs3(Column3, { k: `ss-root-${gameId}`, gap: 24, width: "full", children: [
      /* @__PURE__ */ jsxs3(Row3, { justify: "between", align: "center", width: "full", children: [
        /* @__PURE__ */ jsxs3(Row3, { gap: "sm", align: "center", children: [
          /* @__PURE__ */ jsx4(Text3, { text: "\u6E38\u620F\u622A\u56FE", size: "lg", weight: "semibold", tone: "strong" }),
          /* @__PURE__ */ jsx4(Text3, { text: `\u5171 ${images.length} \u5F20`, size: "xs", tone: "muted" })
        ] }),
        /* @__PURE__ */ jsxs3(Row3, { gap: "xs", align: "center", children: [
          shouldBlurNsfw && sensitiveCount > 0 ? /* @__PURE__ */ jsx4(
            Button,
            {
              k: "ss-toggle-nsfw",
              label: revealNsfw ? "\u9690\u85CF\u654F\u611F\u622A\u56FE" : "\u663E\u793A\u654F\u611F\u622A\u56FE",
              icon: "eye-outline",
              variant: "ghost",
              size: "sm",
              onPress: () => setRevealNsfw((v) => !v)
            }
          ) : null,
          /* @__PURE__ */ jsx4(
            Button,
            {
              k: "ss-refresh",
              label: state.refreshing ? "\u5237\u65B0\u4E2D\u2026" : "\u5237\u65B0",
              icon: "refresh",
              variant: "ghost",
              size: "sm",
              loading: state.refreshing,
              onPress: () => store.loadGame(game, { force: true })
            }
          )
        ] })
      ] }),
      images.length === 0 ? /* @__PURE__ */ jsx4(
        Empty,
        {
          icon: "image-off",
          title: "\u6682\u65E0\u6E38\u620F\u622A\u56FE",
          description: "\u8BE5\u6761\u76EE\u5728 Hikarinagi \u4E0A\u6682\u672A\u6536\u5F55\u622A\u56FE\u3002"
        }
      ) : /* @__PURE__ */ jsx4(Grid, { k: "ss-grid", cols: 2, gap: "md", children: images.map((img, idx) => {
        const sensitive = isMediaSensitive(img);
        const blurred = shouldBlurNsfw && sensitive && !revealNsfw && !revealedMap[idx];
        const dimLabel = img.width && img.height ? `${img.width} \xD7 ${img.height}` : `#${idx + 1}`;
        return /* @__PURE__ */ jsxs3(Column3, { gap: "xs", children: [
          /* @__PURE__ */ jsx4(
            Image,
            {
              k: `ss-img-${idx}`,
              src: img.url,
              height: 216,
              radius: 10,
              fit: "cover",
              blur: blurred,
              previewable: !blurred,
              onPress: blurred ? () => setRevealedMap((prev) => ({ ...prev, [idx]: true })) : void 0
            }
          ),
          /* @__PURE__ */ jsxs3(Row3, { justify: "between", align: "center", width: "full", children: [
            /* @__PURE__ */ jsx4(
              Text3,
              {
                text: blurred ? "\u654F\u611F\u753B\u9762\uFF08\u70B9\u51FB\u89E3\u9664\u6A21\u7CCA\uFF09" : `\u622A\u56FE ${idx + 1}`,
                size: "xs",
                tone: blurred ? "warning" : "faint"
              }
            ),
            /* @__PURE__ */ jsx4(Text3, { text: dimLabel, size: "xs", tone: "faint" })
          ] })
        ] }, `ss-item-${idx}`);
      }) })
    ] });
  };
}

// hikarinagi-detail/src/db.ts
function createCacheDB(ctx) {
  let dbInitPromise = null;
  function ensureDB() {
    if (!dbInitPromise) {
      dbInitPromise = ctx.db.transaction([
        {
          sql: `CREATE TABLE IF NOT EXISTS galgame_cache (
              hikarinagi_id TEXT PRIMARY KEY,
              payload TEXT NOT NULL,
              fetched_at INTEGER NOT NULL,
              updated_at INTEGER NOT NULL
            )`
        },
        {
          sql: `CREATE TABLE IF NOT EXISTS game_bindings (
              local_game_id TEXT PRIMARY KEY,
              hikarinagi_id TEXT NOT NULL,
              manual INTEGER NOT NULL DEFAULT 0,
              matched_title TEXT NOT NULL DEFAULT '',
              updated_at INTEGER NOT NULL
            )`
        },
        {
          sql: `CREATE INDEX IF NOT EXISTS idx_galgame_cache_fetched ON galgame_cache(fetched_at)`
        }
      ]).catch((err) => {
        dbInitPromise = null;
        ctx.logger.error("\u521D\u59CB\u5316 Hikarinagi \u7F13\u5B58\u6570\u636E\u5E93\u5931\u8D25:", err);
        throw err;
      });
    }
    return dbInitPromise;
  }
  async function readCachedDetail(hikarinagiId) {
    await ensureDB();
    const row = await ctx.db.queryOne("SELECT hikarinagi_id, payload, fetched_at FROM galgame_cache WHERE hikarinagi_id = ?", [
      hikarinagiId
    ]);
    if (!row || !row.payload) return null;
    try {
      const detail = JSON.parse(String(row.payload));
      return {
        detail,
        fetchedAt: Number(row.fetched_at || 0)
      };
    } catch {
      return null;
    }
  }
  async function writeCachedDetail(hikarinagiId, detail) {
    await ensureDB();
    const now = Date.now();
    await ctx.db.exec(
      `INSERT INTO galgame_cache (hikarinagi_id, payload, fetched_at, updated_at)
       VALUES (?, ?, ?, ?)
       ON CONFLICT(hikarinagi_id) DO UPDATE SET
         payload = excluded.payload,
         fetched_at = excluded.fetched_at,
         updated_at = excluded.updated_at`,
      [hikarinagiId, JSON.stringify(detail), now, now]
    );
    return now;
  }
  async function readBinding(localGameId) {
    await ensureDB();
    const row = await ctx.db.queryOne(
      "SELECT local_game_id, hikarinagi_id, manual, matched_title, updated_at FROM game_bindings WHERE local_game_id = ?",
      [localGameId]
    );
    if (!row) return null;
    return {
      hikarinagiId: String(row.hikarinagi_id || ""),
      manual: Number(row.manual || 0) === 1,
      matchedTitle: String(row.matched_title || ""),
      updatedAt: Number(row.updated_at || 0)
    };
  }
  async function writeBinding(localGameId, hikarinagiId, manual, matchedTitle) {
    await ensureDB();
    const now = Date.now();
    await ctx.db.exec(
      `INSERT INTO game_bindings (local_game_id, hikarinagi_id, manual, matched_title, updated_at)
       VALUES (?, ?, ?, ?, ?)
       ON CONFLICT(local_game_id) DO UPDATE SET
         hikarinagi_id = excluded.hikarinagi_id,
         manual = excluded.manual,
         matched_title = excluded.matched_title,
         updated_at = excluded.updated_at`,
      [localGameId, hikarinagiId || "", manual ? 1 : 0, matchedTitle || "", now]
    );
  }
  async function deleteBinding(localGameId) {
    await ensureDB();
    await ctx.db.exec("DELETE FROM game_bindings WHERE local_game_id = ?", [localGameId]);
  }
  return {
    readCachedDetail,
    writeCachedDetail,
    readBinding,
    writeBinding,
    deleteBinding
  };
}

// hikarinagi-detail/src/store.ts
import { useEffect as useEffect5, useState as useState5 } from "lunabox";
var NEGATIVE_CACHE_MS = 6 * 60 * 60 * 1e3;
function createEmptyEntry() {
  return {
    status: "idle",
    hikarinagiId: "",
    manualBound: false,
    detail: null,
    fetchedAt: 0,
    fromCache: false,
    refreshing: false,
    error: ""
  };
}
function createDetailStore(ctx, db, api) {
  const entries = /* @__PURE__ */ new Map();
  const inflight = /* @__PURE__ */ new Map();
  const listeners = /* @__PURE__ */ new Map();
  const globalListeners = /* @__PURE__ */ new Set();
  let configVersion = 0;
  function getConfig() {
    return ctx.config.get() || {};
  }
  function getCacheTtlMs() {
    const hours = Number(getConfig().cacheTtlHours);
    const clamped = Number.isFinite(hours) && hours >= 1 ? hours : 24;
    return clamped * 60 * 60 * 1e3;
  }
  function getEntry(gameId) {
    if (!gameId) return createEmptyEntry();
    return entries.get(gameId) || createEmptyEntry();
  }
  function setEntry(gameId, patch) {
    const next = { ...getEntry(gameId), ...patch };
    entries.set(gameId, next);
    const set = listeners.get(gameId);
    if (set) {
      set.forEach((fn) => {
        try {
          fn();
        } catch (e) {
          ctx.logger.error(e);
        }
      });
    }
    return next;
  }
  function subscribeGame(gameId, fn) {
    let set = listeners.get(gameId);
    if (!set) {
      set = /* @__PURE__ */ new Set();
      listeners.set(gameId, set);
    }
    set.add(fn);
    globalListeners.add(fn);
    return () => {
      set.delete(fn);
      if (set.size === 0) listeners.delete(gameId);
      globalListeners.delete(fn);
    };
  }
  ctx.config.onChange(() => {
    configVersion++;
    globalListeners.forEach((fn) => {
      try {
        fn();
      } catch (e) {
        ctx.logger.error(e);
      }
    });
  });
  async function resolveHikarinagiIdForGame(game, forceSearch) {
    const localId = String(game.id);
    const binding = await db.readBinding(localId);
    if (binding && binding.manual && binding.hikarinagiId) {
      return { hikarinagiId: binding.hikarinagiId, manualBound: true };
    }
    if (String(game.source_type || "").toLowerCase() === "hikarinagi") {
      const directId = normalizeHikarinagiId(game.source_id);
      if (directId) {
        return { hikarinagiId: directId, manualBound: false };
      }
    }
    if (binding && !forceSearch) {
      if (binding.hikarinagiId) {
        return { hikarinagiId: binding.hikarinagiId, manualBound: false };
      }
      if (Date.now() - binding.updatedAt < NEGATIVE_CACHE_MS) {
        return { hikarinagiId: "", manualBound: false };
      }
    }
    if (getConfig().autoSearch === false && !forceSearch) {
      return { hikarinagiId: "", manualBound: false };
    }
    const found = await api.searchGalgameId(game);
    if (found && found.hikarinagiId) {
      await db.writeBinding(localId, found.hikarinagiId, false, found.matchedTitle);
      return { hikarinagiId: found.hikarinagiId, manualBound: false };
    }
    await db.writeBinding(localId, "", false, "");
    return { hikarinagiId: "", manualBound: false };
  }
  function loadGame(game, opts) {
    if (!game || !game.id) return Promise.resolve();
    const gameId = String(game.id);
    const force = Boolean(opts?.force);
    const explicitId = opts?.bindId ? normalizeHikarinagiId(opts.bindId) : "";
    if (!force && !explicitId && inflight.has(gameId)) {
      return inflight.get(gameId);
    }
    const current = getEntry(gameId);
    if (!force && !explicitId && current.status === "ready" && current.detail) {
      const age = Date.now() - Number(current.fetchedAt || 0);
      if (age < getCacheTtlMs()) {
        return Promise.resolve();
      }
    }
    let task;
    task = (async () => {
      try {
        let hikarinagiId = explicitId;
        let manualBound = Boolean(explicitId);
        if (explicitId) {
          setEntry(gameId, {
            refreshing: true,
            error: "",
            status: current.detail ? current.status : "loading"
          });
        } else {
          setEntry(gameId, {
            status: current.detail ? current.status : "loading",
            refreshing: Boolean(current.detail),
            error: ""
          });
          const resolved = await resolveHikarinagiIdForGame(game, force);
          hikarinagiId = resolved.hikarinagiId;
          manualBound = resolved.manualBound;
        }
        if (!hikarinagiId) {
          setEntry(gameId, {
            status: "unmatched",
            hikarinagiId: "",
            manualBound: false,
            detail: null,
            fetchedAt: 0,
            fromCache: false,
            refreshing: false,
            error: ""
          });
          return;
        }
        const cached = await db.readCachedDetail(hikarinagiId);
        const ttlMs = getCacheTtlMs();
        const cacheFresh = Boolean(cached?.detail && Date.now() - cached.fetchedAt < ttlMs);
        if (cached?.detail) {
          setEntry(gameId, {
            status: "ready",
            hikarinagiId,
            manualBound,
            detail: cached.detail,
            fetchedAt: cached.fetchedAt,
            fromCache: true,
            refreshing: force || !cacheFresh,
            error: ""
          });
          if (explicitId) {
            const title = cached.detail.trans_title || cached.detail.origin_title || "";
            await db.writeBinding(gameId, hikarinagiId, true, title);
          }
          if (!force && cacheFresh) {
            return;
          }
        }
        const fresh = await api.fetchGalgameDetail(hikarinagiId);
        const fetchedAt = await db.writeCachedDetail(hikarinagiId, fresh);
        if (explicitId) {
          const title = fresh.trans_title || fresh.origin_title || "";
          await db.writeBinding(gameId, hikarinagiId, true, title);
        }
        setEntry(gameId, {
          status: "ready",
          hikarinagiId,
          manualBound,
          detail: fresh,
          fetchedAt,
          fromCache: false,
          refreshing: false,
          error: ""
        });
        if (force || explicitId) {
          ctx.ui.toast("\u5DF2\u4ECE Hikarinagi \u540C\u6B65\u6700\u65B0\u6E38\u620F\u8BE6\u60C5", "success");
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        ctx.logger.warn("\u62C9\u53D6 Hikarinagi \u8BE6\u60C5\u5931\u8D25:", msg);
        const latest = getEntry(gameId);
        if (latest.detail) {
          setEntry(gameId, { refreshing: false, error: msg });
        } else {
          setEntry(gameId, { status: "error", refreshing: false, error: msg });
        }
        if (force || explicitId) {
          ctx.ui.toast(msg, "error");
        }
      } finally {
        if (inflight.get(gameId) === task) {
          inflight.delete(gameId);
        }
      }
    })();
    inflight.set(gameId, task);
    return task;
  }
  async function unbindGame(game) {
    if (!game || !game.id) return;
    const gameId = String(game.id);
    await db.deleteBinding(gameId);
    entries.delete(gameId);
    ctx.ui.toast("\u5DF2\u91CD\u7F6E Hikarinagi \u5173\u8054\uFF0C\u6B63\u5728\u91CD\u65B0\u5339\u914D\u2026", "info");
    await loadGame(game, { force: true });
  }
  function useGameDetail(game) {
    const gameId = game?.id ? String(game.id) : "";
    const [, setTick] = useState5(0);
    useEffect5(() => {
      if (!gameId) return void 0;
      const unsub = subscribeGame(gameId, () => setTick((n) => n + 1));
      loadGame(game, { force: false });
      return unsub;
    }, [gameId]);
    return getEntry(gameId);
  }
  return {
    getConfig,
    loadGame,
    unbindGame,
    useGameDetail
  };
}

// hikarinagi-detail/src/index.tsx
function apply(ctx) {
  ctx.config.define({
    title: "Hikarinagi \u8BE6\u60C5\u9875\u91CD\u6784",
    description: "\u4F7F\u7528 Hikarinagi Galgame \u8BE6\u60C5\u63A5\u53E3\u91CD\u6784\u6E38\u620F\u8BE6\u60C5\u9875\uFF0C\u5E76\u501F\u52A9\u72EC\u7ACB SQLite \u7F13\u5B58\u63A5\u53E3\u6570\u636E\u3002",
    icon: "image-search-outline",
    fields: [
      {
        key: "cacheTtlHours",
        type: "number",
        label: "\u7F13\u5B58\u6709\u6548\u671F\uFF08\u5C0F\u65F6\uFF09",
        description: "\u8D85\u8FC7\u8BE5\u65F6\u957F\u540E\u6253\u5F00\u8BE6\u60C5\u9875\u4F1A\u81EA\u52A8\u5728\u540E\u53F0\u9759\u9ED8\u66F4\u65B0\u7F13\u5B58\u3002",
        default: 24,
        min: 1,
        max: 720,
        step: 1
      },
      {
        key: "autoSearch",
        type: "boolean",
        label: "\u975E Hikarinagi \u6765\u6E90\u81EA\u52A8\u6309\u540D\u79F0\u5339\u914D",
        description: "\u5F53\u6E38\u620F\u6765\u81EA VNDB / Bangumi / Steam \u7B49\u5176\u4ED6\u5143\u6570\u636E\u6765\u6E90\u65F6\uFF0C\u81EA\u52A8\u641C\u7D22\u5E76\u5173\u8054 Hikarinagi \u6761\u76EE\u3002",
        default: true
      },
      {
        key: "preferredLogoLang",
        type: "enum",
        label: "\u4F18\u5148\u5FBD\u6807\u8BED\u8A00",
        default: "auto",
        options: [
          { value: "auto", label: "\u8DDF\u968F\u5E94\u7528\u8BED\u8A00" },
          { value: "zh", label: "\u4F18\u5148\u4E2D\u6587\u5FBD\u6807" },
          { value: "ja", label: "\u4F18\u5148\u65E5\u6587\u5FBD\u6807" },
          { value: "en", label: "\u4F18\u5148\u82F1\u6587\u5FBD\u6807" }
        ]
      },
      {
        key: "heroHeight",
        type: "number",
        label: "\u5934\u56FE\u533A\u57DF\u9AD8\u5EA6\uFF08px\uFF09",
        description: "\u63A7\u5236\u9876\u90E8\u5168\u51FA\u8840\u5934\u56FE\u6A2A\u5E45\u7684\u6700\u5C0F\u9AD8\u5EA6\uFF0C\u63A8\u8350 520\u2013620px\u3002",
        default: 560,
        min: 360,
        max: 760,
        step: 10
      },
      {
        key: "blurHeroBanner",
        type: "boolean",
        label: "\u6A21\u7CCA\u5934\u56FE\u80CC\u666F",
        description: "\u9ED8\u8BA4\u6E05\u6670\u5C55\u793A\u539F\u59CB\u9AD8\u6E05\u5934\u56FE\uFF1B\u5F00\u542F\u540E\u5BF9\u6240\u6709\u5934\u56FE\u65BD\u52A0\u9AD8\u65AF\u6A21\u7CCA\u3002",
        default: false
      },
      {
        key: "accessToken",
        type: "secret",
        label: "\u81EA\u5B9A\u4E49 Hikarinagi Token\uFF08\u53EF\u9009\uFF09",
        description: "\u7559\u7A7A\u65F6\u81EA\u52A8\u4F7F\u7528 LunaBox \u5DF2\u767B\u5F55\u8D26\u53F7\u6216\u5E94\u7528\u5185\u7F6E\u7684 Hikarinagi \u8BBF\u95EE\u4EE4\u724C\u3002",
        default: ""
      }
    ]
  });
  const db = createCacheDB(ctx);
  const api = createHikarinagiApi(ctx);
  const store = createDetailStore(ctx, db, api);
  ctx.ui.slot("game.detail.hero", {
    id: "hikarinagi-hero",
    hideTabs: ["progress"],
    render: createHeroSection(ctx, store)
  });
  ctx.ui.slot("game.detail.tab", {
    id: "overview",
    title: "\u6982\u89C8",
    icon: "file-document-outline",
    order: -10,
    render: createOverviewTab(store)
  });
  ctx.ui.slot("game.detail.tab", {
    id: "screenshots",
    title: "\u6E38\u620F\u622A\u56FE",
    icon: "image-multiple-outline",
    order: 1,
    render: createScreenshotsTab(store)
  });
  ctx.ui.slot("game.detail.aside", {
    id: "detail-info",
    render: createDetailAside(ctx, store)
  });
}
export {
  apply
};
