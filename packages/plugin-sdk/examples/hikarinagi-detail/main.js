// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/format.ts
var SITE_GALGAME_BASE = "https://www.hikarinagi.org/galgames/";
function normalizeHikarinagiId(input) {
  if (input == null)
    return "";
  const raw = String(input).trim();
  if (!raw)
    return "";
  const urlMatch = raw.match(/hikarinagi\.org\/galgames?\/(\d+)/i);
  const candidate = urlMatch ? urlMatch[1] : raw.replace(/^#/, "");
  if (!/^\d+$/.test(candidate))
    return "";
  const num = Number(candidate);
  if (!Number.isFinite(num) || num <= 0)
    return "";
  return String(Math.floor(num));
}
function normalizeText(s) {
  return String(s || "").toLowerCase().replace(/[\s　\-_~～·•・:：!！?？.,，。'"`「」『』【】()（）\[\]]+/g, "");
}
function formatReleaseDate(detail) {
  if (!detail)
    return "";
  if (detail.release_date_tbd) {
    return detail.release_date_tbd_note ? `待定（${detail.release_date_tbd_note}）` : "发售日待定";
  }
  const raw = String(detail.release_date || "").trim();
  if (!raw)
    return "";
  const datePart = raw.slice(0, 10);
  const precision = String(detail.release_date_precision || "DAY").toUpperCase();
  if (precision === "YEAR" && datePart.length >= 4) {
    return `${datePart.slice(0, 4)} 年`;
  }
  if (precision === "MONTH" && datePart.length >= 7) {
    return `${datePart.slice(0, 4)} 年 ${Number(datePart.slice(5, 7))} 月`;
  }
  if (datePart.length === 10 && datePart[4] === "-" && datePart[7] === "-") {
    return `${datePart.slice(0, 4)} 年 ${Number(datePart.slice(5, 7))} 月 ${Number(datePart.slice(8, 10))} 日`;
  }
  return datePart;
}
function formatStatusLabel(detail) {
  if (!detail)
    return "";
  const st = String(detail.release_status || detail.dev_status || "").toUpperCase();
  switch (st) {
    case "RELEASED":
      return "已发售";
    case "PLANNED":
    case "IN_DEVELOPMENT":
      return "开发中 / 预定发售";
    case "CANCELLED":
      return "已取消";
    default:
      return st || "";
  }
}
function formatLangLabel(lang) {
  const code = String(lang || "").trim().toLowerCase();
  if (!code)
    return "";
  const map = {
    ja: "日语",
    jp: "日语",
    "ja-jp": "日语",
    zh: "中文",
    "zh-cn": "简体中文",
    "zh-hans": "简体中文",
    "zh-tw": "繁体中文",
    "zh-hant": "繁体中文",
    en: "英语",
    "en-us": "英语",
    ko: "韩语",
    "ko-kr": "韩语"
  };
  return map[code] || String(lang || "");
}
function formatRelativeTime(tsMs) {
  if (!tsMs)
    return "";
  const diffSec = Math.max(0, Math.floor((Date.now() - tsMs) / 1000));
  if (diffSec < 60)
    return "刚刚";
  if (diffSec < 3600)
    return `${Math.floor(diffSec / 60)} 分钟前`;
  if (diffSec < 86400)
    return `${Math.floor(diffSec / 3600)} 小时前`;
  return `${Math.floor(diffSec / 86400)} 天前`;
}
function formatPrice(p) {
  if (!p)
    return "";
  const parts = [];
  if (p.version)
    parts.push(String(p.version));
  const amount = p.amount != null ? Number(p.amount).toLocaleString() : "";
  const currency = p.currency ? String(p.currency) : "";
  const money = [amount, currency].filter(Boolean).join(" ");
  if (money)
    parts.push(money);
  if (p.tax_included === true)
    parts.push("含税");
  else if (p.tax_included === false)
    parts.push("不含税");
  return parts.join(" · ");
}
function isMediaSensitive(item) {
  if (!item)
    return false;
  return Number(item.sexual || 0) > 0 || Number(item.violence || 0) > 0;
}
function pickBestLogo(logos, preferredLang, appLang) {
  if (!Array.isArray(logos) || logos.length === 0)
    return null;
  const valid = logos.filter((l) => l && l.url);
  if (valid.length === 0)
    return null;
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
    if (hit)
      return hit;
  }
  return valid[0];
}
function sortHeroImages(images, blurNsfw) {
  if (!Array.isArray(images))
    return [];
  const valid = images.filter((img) => img && img.url);
  const scoreImage = (img) => {
    const w = Number(img.width || 0);
    const h = Number(img.height || 0);
    const ratio = w > 0 && h > 0 ? w / h : 1.77;
    const isLandscape = ratio >= 1.25;
    const sensitive = isMediaSensitive(img);
    let score = 0;
    if (isLandscape)
      score += 1e4;
    if (!sensitive)
      score += blurNsfw ? 50000 : 5000;
    else if (blurNsfw)
      score -= 20000;
    score -= Math.abs(ratio - 1.78) * 1200;
    score += Math.min(w * h, 2560 * 1440) / 1000;
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
    const safeCover = blurNsfw ? covers.find((c) => !isMediaSensitive(c)) : undefined;
    const picked = safeCover || covers[0];
    return {
      url: picked.url,
      sensitive: isMediaSensitive(picked) || Boolean(detail?.nsfw),
      fromCover: true
    };
  }
  const fallback = game?.cover_source_url || game?.cover_url || "";
  return {
    url: fallback,
    sensitive: Boolean(game?.is_nsfw),
    fromCover: true
  };
}
function pickBestSearchHit(items, game) {
  const hits = (Array.isArray(items) ? items : []).filter((item) => item && item.type === "galgame" && Number(item.id) > 0);
  if (hits.length === 0)
    return null;
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
    if (byCompany)
      return byCompany;
  }
  return hits[0];
}
function buildBangumiUrl(raw) {
  const v = String(raw ?? "").trim();
  if (!v)
    return "";
  if (/^https?:\/\//i.test(v))
    return v;
  const digits = v.replace(/[^\d]/g, "");
  return digits ? `https://bgm.tv/subject/${digits}` : "";
}
function buildVndbUrl(raw) {
  const v = String(raw ?? "").trim();
  if (!v)
    return "";
  if (/^https?:\/\//i.test(v))
    return v;
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

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/api.ts
var API_BASE = "https://api.hikarinagi.org/v3";
function createHikarinagiApi(ctx) {
  async function resolveToken() {
    const cfg = ctx.config.get() || {};
    const custom = String(cfg.accessToken || "").trim();
    if (custom)
      return custom;
    const token = await ctx.metadata.getAuthToken("hikarinagi");
    return String(token || "").trim();
  }
  async function apiGet(path) {
    const token = await resolveToken();
    if (!token) {
      throw new Error("未获取到 Hikarinagi 访问令牌，请先在设置中登录 Hikarinagi 或填写自定义 Token");
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
      throw new Error(`Hikarinagi API 请求失败：${msg}`);
    }
    if (!json || json.success === false) {
      const msg = json?.message || "接口返回异常";
      throw new Error(`Hikarinagi API 错误：${msg}`);
    }
    return json.data;
  }
  async function fetchGalgameDetail(hikarinagiId) {
    const data = await apiGet(`/galgames/${encodeURIComponent(hikarinagiId)}`);
    if (!data || !data.id) {
      throw new Error(`未找到 Hikarinagi 条目 #${hikarinagiId}`);
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
        if (queries.length >= 3)
          break;
        const alias = String(raw || "").trim();
        if (alias && !queries.includes(alias)) {
          queries.push(alias);
        }
      }
    }
    for (const q of queries) {
      const data = await apiGet(`/search?q=${encodeURIComponent(q)}&types=galgame&page=1&page_size=5`);
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

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/components/DetailAside.tsx
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
  useState
} from "lunabox";
import { jsx, jsxs, Fragment } from "lunabox/jsx-runtime";
function InfoRow({
  label,
  value
}) {
  const v = String(value || "").trim();
  if (!v)
    return null;
  return /* @__PURE__ */ jsxs(Row, {
    gap: "sm",
    align: "start",
    width: "full",
    children: [
      /* @__PURE__ */ jsx(Box, {
        width: 64,
        shrink: 0,
        children: /* @__PURE__ */ jsx(Text, {
          text: label,
          size: "xs",
          tone: "muted"
        })
      }),
      /* @__PURE__ */ jsx(Box, {
        grow: 1,
        basis: 0,
        minWidth: 0,
        children: /* @__PURE__ */ jsx(Text, {
          text: v,
          size: "xs",
          tone: "strong",
          selectable: true
        })
      })
    ]
  });
}
function RatingStars({ score }) {
  const stars = score / 2;
  return /* @__PURE__ */ jsx(Row, {
    gap: 2,
    align: "center",
    children: [1, 2, 3, 4, 5].map((i) => {
      let name = "star-outline";
      let tone = "faint";
      if (stars >= i - 0.25) {
        name = "star";
        tone = "warning";
      } else if (stars >= i - 0.75) {
        name = "star-half-full";
        tone = "warning";
      }
      return /* @__PURE__ */ jsx(Icon, {
        name,
        size: 15,
        tone
      }, `st-${i}`);
    })
  });
}
function RatingDistribution({ rating }) {
  if (!rating?.count || Number(rating.total || 0) <= 0)
    return null;
  const total = Number(rating.total);
  const c = rating.count;
  const buckets = [
    { label: "9-10", n: Number(c["10"] || 0) + Number(c["9"] || 0) },
    { label: "7-8", n: Number(c["8"] || 0) + Number(c["7"] || 0) },
    { label: "5-6", n: Number(c["6"] || 0) + Number(c["5"] || 0) },
    { label: "3-4", n: Number(c["4"] || 0) + Number(c["3"] || 0) },
    { label: "1-2", n: Number(c["2"] || 0) + Number(c["1"] || 0) }
  ];
  return /* @__PURE__ */ jsx(Column, {
    k: "aside-rating-dist",
    gap: 6,
    width: "full",
    marginTop: "xs",
    children: buckets.map((b, idx) => {
      const pct = total > 0 ? Math.round(b.n / total * 100) : 0;
      return /* @__PURE__ */ jsxs(Row, {
        gap: "sm",
        align: "center",
        width: "full",
        children: [
          /* @__PURE__ */ jsx(Box, {
            width: 34,
            shrink: 0,
            children: /* @__PURE__ */ jsx(Text, {
              text: b.label,
              size: "xs",
              tone: "faint"
            })
          }),
          /* @__PURE__ */ jsx(Box, {
            grow: 1,
            basis: 0,
            minWidth: 0,
            children: /* @__PURE__ */ jsx(Progress, {
              value: pct,
              max: 100,
              height: 5,
              tone: idx === 0 ? "brand" : "default"
            })
          }),
          /* @__PURE__ */ jsx(Box, {
            width: 30,
            shrink: 0,
            children: /* @__PURE__ */ jsx(Text, {
              text: `${pct}%`,
              size: "xs",
              tone: "faint"
            })
          })
        ]
      }, `rd-${idx}`);
    })
  });
}
function createDetailAside(ctx, store) {
  return function DetailAside({ game }) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : "";
    const [bindInput, setBindInput] = useState("");
    useEffect(() => {
      setBindInput("");
    }, [gameId]);
    const handleManualBind = () => {
      const normalized = normalizeHikarinagiId(bindInput);
      if (!normalized) {
        ctx.ui.toast("请输入有效的 Hikarinagi 数字 ID 或条目链接", "error");
        return;
      }
      setBindInput("");
      store.loadGame(game, { force: true, bindId: normalized });
    };
    if (state.status === "loading" && !detail) {
      return /* @__PURE__ */ jsxs(Column, {
        k: "aside-loading",
        gap: "md",
        width: "full",
        children: [
          /* @__PURE__ */ jsxs(Row, {
            justify: "between",
            align: "center",
            width: "full",
            children: [
              /* @__PURE__ */ jsx(Text, {
                text: "游戏详情",
                size: "base",
                weight: "semibold",
                tone: "strong"
              }),
              /* @__PURE__ */ jsx(Text, {
                text: "同步中…",
                size: "xs",
                tone: "muted"
              })
            ]
          }),
          /* @__PURE__ */ jsx(Skeleton, {
            lines: 7,
            height: 14
          })
        ]
      });
    }
    const links = [];
    if (detail?.id) {
      links.push({
        label: `Hikarinagi #${detail.id}`,
        url: `${SITE_GALGAME_BASE}${detail.id}`,
        icon: "open-in-new"
      });
    }
    if (detail?.homepage) {
      links.push({ label: "官方网站", url: String(detail.homepage), icon: "link-variant" });
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
      if (bgmUrl)
        links.push({ label: "Bangumi", url: bgmUrl, icon: "open-in-new" });
      const vndbUrl = buildVndbUrl(detail.external_source.vndb);
      if (vndbUrl)
        links.push({ label: "VNDB", url: vndbUrl, icon: "open-in-new" });
    }
    if (Array.isArray(detail?.external_links)) {
      detail.external_links.forEach((el) => {
        if (el?.url) {
          links.push({
            label: String(el.label || el.name || "外部链接"),
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
        if (lbl && !langs.includes(lbl))
          langs.push(lbl);
      }
    } else if (detail?.origin_lang) {
      const lbl = formatLangLabel(detail.origin_lang);
      if (lbl)
        langs.push(lbl);
    }
    const score = Number(detail?.rating?.score || 0);
    const total = Number(detail?.rating?.total || 0);
    const played = Number(detail?.rating?.played || 0);
    const cacheLabel = state.fetchedAt ? `${state.fromCache ? "SQLite 缓存" : "实时同步"} · ${formatRelativeTime(state.fetchedAt)}` : "";
    return /* @__PURE__ */ jsxs(Column, {
      k: `aside-root-${gameId}`,
      gap: 20,
      width: "full",
      children: [
        detail ? /* @__PURE__ */ jsxs(Fragment, {
          children: [
            /* @__PURE__ */ jsxs(Column, {
              k: "aside-rating-sec",
              gap: "sm",
              width: "full",
              children: [
                /* @__PURE__ */ jsxs(Row, {
                  justify: "between",
                  align: "center",
                  width: "full",
                  children: [
                    /* @__PURE__ */ jsxs(Row, {
                      gap: "xs",
                      align: "center",
                      children: [
                        /* @__PURE__ */ jsx(Text, {
                          text: "社区评分",
                          size: "sm",
                          weight: "semibold",
                          tone: "strong"
                        }),
                        detail.nsfw ? /* @__PURE__ */ jsx(Badge, {
                          label: "R-18",
                          tone: "danger"
                        }) : null
                      ]
                    }),
                    /* @__PURE__ */ jsxs(Row, {
                      gap: "xs",
                      align: "center",
                      children: [
                        /* @__PURE__ */ jsx(IconButton, {
                          k: "aside-refresh-btn",
                          icon: "refresh",
                          size: "sm",
                          label: "刷新 Hikarinagi 详情",
                          disabled: state.refreshing,
                          onPress: () => store.loadGame(game, { force: true })
                        }),
                        /* @__PURE__ */ jsx(IconButton, {
                          k: "aside-open-hn",
                          icon: "open-in-new",
                          size: "sm",
                          label: "在浏览器打开 Hikarinagi",
                          onPress: () => ctx.shell.openURL(`${SITE_GALGAME_BASE}${detail.id}`)
                        })
                      ]
                    })
                  ]
                }),
                score > 0 ? /* @__PURE__ */ jsxs(Row, {
                  gap: "md",
                  align: "center",
                  width: "full",
                  children: [
                    /* @__PURE__ */ jsx(Text, {
                      text: score.toFixed(1),
                      size: "3xl",
                      weight: "bold",
                      tone: "strong"
                    }),
                    /* @__PURE__ */ jsxs(Column, {
                      gap: 2,
                      children: [
                        /* @__PURE__ */ jsx(RatingStars, {
                          score
                        }),
                        /* @__PURE__ */ jsx(Text, {
                          text: [total > 0 ? `${total} 人评分` : "", played > 0 ? `${played} 人玩过` : ""].filter(Boolean).join(" · ") || "Hikarinagi 均分",
                          size: "xs",
                          tone: "muted"
                        })
                      ]
                    })
                  ]
                }) : /* @__PURE__ */ jsx(Text, {
                  text: "暂无社区评分",
                  size: "sm",
                  tone: "muted"
                }),
                /* @__PURE__ */ jsx(RatingDistribution, {
                  rating: detail.rating
                }),
                cacheLabel ? /* @__PURE__ */ jsx(Text, {
                  text: cacheLabel,
                  size: "xs",
                  tone: "faint"
                }) : null
              ]
            }),
            /* @__PURE__ */ jsx(Divider, {
              subtle: true
            })
          ]
        }) : null,
        /* @__PURE__ */ jsxs(Column, {
          k: "aside-info-sec",
          gap: "md",
          width: "full",
          children: [
            /* @__PURE__ */ jsxs(Row, {
              justify: "between",
              align: "center",
              width: "full",
              children: [
                /* @__PURE__ */ jsx(Text, {
                  text: "游戏信息",
                  size: "sm",
                  weight: "semibold",
                  tone: "strong"
                }),
                detail ? /* @__PURE__ */ jsx(Text, {
                  text: `#${detail.id}`,
                  size: "xs",
                  tone: "faint"
                }) : /* @__PURE__ */ jsx(Text, {
                  text: state.status === "error" ? "同步失败" : "未匹配",
                  size: "xs",
                  tone: "warning"
                })
              ]
            }),
            detail ? /* @__PURE__ */ jsxs(Column, {
              k: "aside-kv-list",
              gap: 10,
              width: "full",
              children: [
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "原名",
                  value: detail.origin_title
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "译名",
                  value: detail.trans_title
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "英文名",
                  value: detail.en_title
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "开发",
                  value: detail.developer
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "发行",
                  value: detail.publisher
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "首发",
                  value: formatReleaseDate(detail)
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "状态",
                  value: formatStatusLabel(detail)
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "类型",
                  value: detail.adv_type
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "平台",
                  value: platforms.join("、")
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "语言",
                  value: langs.join("、")
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "引擎",
                  value: detail.engine
                }),
                /* @__PURE__ */ jsx(InfoRow, {
                  label: "分级",
                  value: detail.nsfw ? "R-18 / 成人向" : "全年龄"
                }),
                prices.length > 0 ? /* @__PURE__ */ jsx(InfoRow, {
                  label: "定价",
                  value: prices.slice(0, 3).join(`
`)
                }) : null
              ]
            }) : /* @__PURE__ */ jsxs(Column, {
              gap: "sm",
              width: "full",
              children: [
                /* @__PURE__ */ jsx(Text, {
                  text: state.error || "尚未自动匹配到 Hikarinagi 条目，可在下方输入条目 ID 手动关联。",
                  size: "xs",
                  tone: state.status === "error" ? "danger" : "muted"
                }),
                game?.company ? /* @__PURE__ */ jsx(InfoRow, {
                  label: "开发",
                  value: game.company
                }) : null,
                game?.release_date ? /* @__PURE__ */ jsx(InfoRow, {
                  label: "首发",
                  value: game.release_date
                }) : null
              ]
            })
          ]
        }),
        links.length > 0 ? /* @__PURE__ */ jsxs(Fragment, {
          children: [
            /* @__PURE__ */ jsx(Divider, {
              subtle: true
            }),
            /* @__PURE__ */ jsxs(Column, {
              k: "aside-links-sec",
              gap: "sm",
              width: "full",
              children: [
                /* @__PURE__ */ jsx(Text, {
                  text: "相关链接",
                  size: "sm",
                  weight: "semibold",
                  tone: "strong"
                }),
                /* @__PURE__ */ jsx(Row, {
                  gap: "xs",
                  wrap: true,
                  width: "full",
                  children: links.map((lk, idx) => /* @__PURE__ */ jsx(Button, {
                    label: lk.label,
                    icon: lk.icon,
                    variant: "secondary",
                    size: "sm",
                    onPress: () => ctx.shell.openURL(lk.url)
                  }, `ext-link-${idx}`))
                })
              ]
            })
          ]
        }) : null,
        /* @__PURE__ */ jsx(Divider, {
          subtle: true
        }),
        /* @__PURE__ */ jsx(Collapsible, {
          k: "aside-bind-col",
          title: "条目关联与缓存",
          icon: "database-refresh-outline",
          defaultOpen: !detail,
          children: /* @__PURE__ */ jsxs(Column, {
            gap: "sm",
            width: "full",
            paddingY: "xs",
            children: [
              /* @__PURE__ */ jsx(Text, {
                text: "输入 Hikarinagi 数字 ID 或详情页链接：",
                size: "xs",
                tone: "muted"
              }),
              /* @__PURE__ */ jsxs(Row, {
                gap: "xs",
                align: "center",
                width: "full",
                children: [
                  /* @__PURE__ */ jsx(Box, {
                    grow: 1,
                    basis: 0,
                    minWidth: 0,
                    children: /* @__PURE__ */ jsx(Input, {
                      k: "aside-bind-input",
                      value: bindInput,
                      placeholder: "例如 25423 或条目 URL",
                      onChange: setBindInput,
                      onSubmit: handleManualBind
                    })
                  }),
                  /* @__PURE__ */ jsx(Button, {
                    k: "aside-bind-submit",
                    label: "绑定",
                    variant: "primary",
                    size: "sm",
                    loading: state.refreshing,
                    onPress: handleManualBind
                  })
                ]
              }),
              /* @__PURE__ */ jsxs(Row, {
                gap: "xs",
                wrap: true,
                children: [
                  /* @__PURE__ */ jsx(Button, {
                    k: "aside-force-refresh",
                    label: "强制刷新",
                    icon: "refresh",
                    variant: "ghost",
                    size: "sm",
                    loading: state.refreshing,
                    onPress: () => store.loadGame(game, { force: true })
                  }),
                  state.manualBound ? /* @__PURE__ */ jsx(Button, {
                    k: "aside-unbind",
                    label: "取消手动绑定",
                    icon: "restore",
                    variant: "ghost",
                    size: "sm",
                    onPress: () => store.unbindGame(game)
                  }) : null
                ]
              })
            ]
          })
        })
      ]
    });
  };
}

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/components/HeroSection.tsx
import { Box as Box2, HeroBanner, Image, Text as Text2, useEffect as useEffect2, useState as useState2 } from "lunabox";
import { jsx as jsx2 } from "lunabox/jsx-runtime";
function createHeroSection(ctx, store) {
  return function HeroSection({ game, blurNsfw }) {
    const state = store.useGameDetail(game);
    const cfg = store.getConfig();
    const detail = state.detail;
    const shouldBlurNsfw = Boolean(blurNsfw);
    const gameId = game?.id ? String(game.id) : "";
    const [selectedIdx, setSelectedIdx] = useState2(-1);
    useEffect2(() => {
      setSelectedIdx(-1);
    }, [gameId]);
    const sortedImages = sortHeroImages(detail?.images, shouldBlurNsfw);
    const defaultBanner = pickBestBanner(detail, game, shouldBlurNsfw);
    const activeImg = selectedIdx >= 0 && selectedIdx < sortedImages.length ? sortedImages[selectedIdx] : null;
    const activeBannerUrl = activeImg ? activeImg.url : defaultBanner.url;
    const activeBannerSensitive = activeImg ? isMediaSensitive(activeImg) : defaultBanner.sensitive;
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
      meta.push({ icon: "gamepad-variant-outline", text: platforms.slice(0, 3).join("、") });
    }
    const rawLangs = [];
    if (Array.isArray(detail?.languages) && detail.languages.length > 0) {
      for (const l of detail.languages) {
        const lbl = formatLangLabel(l);
        if (lbl && !rawLangs.includes(lbl))
          rawLangs.push(lbl);
      }
    } else if (detail?.origin_lang) {
      const lbl = formatLangLabel(detail.origin_lang);
      if (lbl)
        rawLangs.push(lbl);
    }
    if (rawLangs.length > 0) {
      meta.push({ icon: "earth", text: rawLangs.slice(0, 4).join("、") });
    } else if (detail?.adv_type) {
      meta.push({ icon: "tag-outline", text: String(detail.adv_type) });
    }
    const hnScore = Number(detail?.rating?.score || 0);
    const localScore = Number(game?.rating || 0);
    const rating = hnScore > 0 ? {
      score: hnScore,
      total: Number(detail?.rating?.total || 0),
      played: Number(detail?.rating?.played || 0)
    } : localScore > 0 ? { score: localScore, total: 0, played: 0 } : undefined;
    const rawHeight = Number(cfg.heroHeight);
    const heroHeight = Number.isFinite(rawHeight) && rawHeight >= 320 ? Math.min(580, rawHeight) : 460;
    const shouldBlurHero = Boolean(cfg.blurHeroBanner) || shouldBlurNsfw && activeBannerSensitive;
    const stripImages = sortedImages.slice(0, 6);
    const currentEffectiveIdx = selectedIdx >= 0 ? selectedIdx : 0;
    return /* @__PURE__ */ jsx2(HeroBanner, {
      k: `hero-${gameId}`,
      height: heroHeight,
      banner: activeBannerUrl,
      fallbackBanner: game?.cover_source_url || game?.cover_url || "",
      logo: logo?.url || "",
      title,
      subtitle,
      meta,
      rating,
      blur: shouldBlurHero,
      backSlot: "back",
      coverSlot: "cover",
      toolbarSlot: "toolbar",
      children: stripImages.length > 1 ? [
        ...stripImages.map((img, idx) => {
          const isActive = idx === currentEffectiveIdx;
          const thumbBlur = shouldBlurNsfw && isMediaSensitive(img);
          return /* @__PURE__ */ jsx2(Box2, {
            radius: "sm",
            opacity: isActive ? 1 : 0.78,
            children: /* @__PURE__ */ jsx2(Image, {
              k: `hero-thumb-${idx}`,
              src: img.url,
              width: 122,
              height: 68,
              radius: 7,
              fit: "cover",
              blur: thumbBlur,
              hoverBorder: true,
              previewable: isActive && !thumbBlur,
              onPress: isActive ? undefined : () => {
                setSelectedIdx(idx);
              }
            })
          }, `hero-thumb-wrap-${idx}`);
        }),
        sortedImages.length > stripImages.length ? /* @__PURE__ */ jsx2(Box2, {
          paddingX: "sm",
          paddingY: "xs",
          children: /* @__PURE__ */ jsx2(Text2, {
            text: `共 ${sortedImages.length} 张截图`,
            size: "xs",
            tone: "light-muted"
          })
        }, "hero-thumb-more") : null
      ] : null
    });
  };
}

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/components/OverviewTab.tsx
import { Badge as Badge2, Button as Button2, Column as Column2, Divider as Divider2, NativeSlot, Row as Row2, Skeleton as Skeleton2, Text as Text3, useEffect as useEffect3, useState as useState3 } from "lunabox";
import { jsx as jsx3, jsxs as jsxs2, Fragment as Fragment2 } from "lunabox/jsx-runtime";
function createOverviewTab(store) {
  return function OverviewTab({ game }) {
    const state = store.useGameDetail(game);
    const detail = state.detail;
    const gameId = game?.id ? String(game.id) : "";
    const [introLang, setIntroLang] = useState3("zh");
    useEffect3(() => {
      setIntroLang("zh");
    }, [gameId]);
    const transIntro = String(detail?.trans_intro || "").trim();
    const originIntro = String(detail?.origin_intro || "").trim();
    const hasBothIntros = Boolean(transIntro && originIntro && transIntro !== originIntro);
    const activeIntro = introLang === "origin" ? originIntro || transIntro : transIntro || originIntro;
    const sortedTags = Array.isArray(detail?.tags) ? detail.tags.filter((t) => t && t.name).slice().sort((a, b) => Number(b.likes || 0) - Number(a.likes || 0)).slice(0, 24) : [];
    const aliases = Array.isArray(detail?.aliases) ? detail.aliases.map((a) => String(a || "").trim()).filter(Boolean) : [];
    return /* @__PURE__ */ jsxs2(Column2, {
      k: `overview-${gameId}`,
      gap: 24,
      width: "full",
      children: [
        /* @__PURE__ */ jsxs2(Column2, {
          k: "ov-intro-sec",
          gap: "md",
          width: "full",
          children: [
            /* @__PURE__ */ jsxs2(Row2, {
              justify: "between",
              align: "center",
              width: "full",
              children: [
                /* @__PURE__ */ jsx3(Text3, {
                  text: "简介",
                  size: "lg",
                  weight: "semibold",
                  tone: "strong"
                }),
                hasBothIntros ? /* @__PURE__ */ jsxs2(Row2, {
                  gap: "xs",
                  align: "center",
                  children: [
                    /* @__PURE__ */ jsx3(Button2, {
                      k: "ov-lang-zh",
                      label: "中文",
                      size: "sm",
                      variant: introLang === "zh" ? "secondary" : "ghost",
                      onPress: () => setIntroLang("zh")
                    }),
                    /* @__PURE__ */ jsx3(Button2, {
                      k: "ov-lang-origin",
                      label: "原文",
                      size: "sm",
                      variant: introLang === "origin" ? "secondary" : "ghost",
                      onPress: () => setIntroLang("origin")
                    })
                  ]
                }) : null
              ]
            }),
            state.status === "loading" && !detail && !game?.summary ? /* @__PURE__ */ jsx3(Skeleton2, {
              lines: 4,
              height: 14
            }) : activeIntro ? /* @__PURE__ */ jsx3(Text3, {
              text: activeIntro,
              size: "sm",
              tone: "default",
              selectable: true
            }) : /* @__PURE__ */ jsx3(NativeSlot, {
              k: "ov-native-summary",
              name: "summary"
            })
          ]
        }),
        /* @__PURE__ */ jsx3(Divider2, {
          subtle: true
        }),
        /* @__PURE__ */ jsxs2(Column2, {
          k: "ov-tags-sec",
          gap: "md",
          width: "full",
          children: [
            /* @__PURE__ */ jsx3(Text3, {
              text: "标签",
              size: "lg",
              weight: "semibold",
              tone: "strong"
            }),
            /* @__PURE__ */ jsx3(NativeSlot, {
              k: "ov-native-tags",
              name: "tags"
            }),
            sortedTags.length > 0 ? /* @__PURE__ */ jsxs2(Column2, {
              k: "ov-hn-tags",
              gap: "sm",
              width: "full",
              marginTop: "xs",
              children: [
                /* @__PURE__ */ jsx3(Text3, {
                  text: `Hikarinagi 社区标签 (${sortedTags.length})`,
                  size: "xs",
                  weight: "medium",
                  tone: "muted"
                }),
                /* @__PURE__ */ jsx3(Row2, {
                  gap: "xs",
                  wrap: true,
                  width: "full",
                  children: sortedTags.map((tg, idx) => /* @__PURE__ */ jsx3(Badge2, {
                    label: Number(tg.likes) > 0 ? `${tg.name} · ${tg.likes}` : tg.name,
                    tone: idx < 4 ? "brand" : "default"
                  }, `ov-htag-${idx}`))
                })
              ]
            }) : null
          ]
        }),
        aliases.length > 0 ? /* @__PURE__ */ jsxs2(Fragment2, {
          children: [
            /* @__PURE__ */ jsx3(Divider2, {
              subtle: true
            }),
            /* @__PURE__ */ jsxs2(Column2, {
              k: "ov-aliases-sec",
              gap: "sm",
              width: "full",
              children: [
                /* @__PURE__ */ jsx3(Text3, {
                  text: "别名",
                  size: "base",
                  weight: "semibold",
                  tone: "strong"
                }),
                /* @__PURE__ */ jsx3(Row2, {
                  gap: "sm",
                  wrap: true,
                  width: "full",
                  children: aliases.map((al, idx) => /* @__PURE__ */ jsx3(Text3, {
                    text: idx < aliases.length - 1 ? `${al}  /` : al,
                    size: "sm",
                    tone: "muted",
                    selectable: true
                  }, `ov-alias-${idx}`))
                })
              ]
            })
          ]
        }) : null
      ]
    });
  };
}

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/components/ScreenshotsTab.tsx
import {
  Button as Button3,
  Collapsible as Collapsible2,
  Column as Column3,
  Divider as Divider3,
  Empty,
  Grid,
  Image as Image2,
  Row as Row3,
  Skeleton as Skeleton3,
  Text as Text4,
  useEffect as useEffect4,
  useState as useState4
} from "lunabox";
import { jsx as jsx4, jsxs as jsxs3, Fragment as Fragment3 } from "lunabox/jsx-runtime";
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
      return /* @__PURE__ */ jsxs3(Column3, {
        k: "ss-loading",
        gap: "md",
        width: "full",
        children: [
          /* @__PURE__ */ jsx4(Text4, {
            text: "正在加载游戏截图…",
            size: "sm",
            tone: "muted"
          }),
          /* @__PURE__ */ jsx4(Skeleton3, {
            lines: 5,
            height: 18
          })
        ]
      });
    }
    if (!detail) {
      return /* @__PURE__ */ jsx4(Empty, {
        k: "ss-unmatched",
        icon: "image-off",
        title: state.status === "error" ? "获取 Hikarinagi 截图失败" : "未匹配到 Hikarinagi 条目",
        description: state.error || "请在右侧「关联与缓存」中手动输入 Hikarinagi 条目 ID，或点击重试。",
        actionLabel: "重新匹配",
        onAction: () => store.loadGame(game, { force: true })
      });
    }
    const images = Array.isArray(detail.images) ? detail.images.filter((i) => i && i.url) : [];
    const covers = Array.isArray(detail.covers) ? detail.covers.filter((c) => c && c.url) : [];
    const logos = Array.isArray(detail.logos) ? detail.logos.filter((l) => l && l.url) : [];
    const sensitiveCount = images.filter(isMediaSensitive).length;
    return /* @__PURE__ */ jsxs3(Column3, {
      k: `ss-root-${gameId}`,
      gap: 24,
      width: "full",
      children: [
        /* @__PURE__ */ jsxs3(Row3, {
          justify: "between",
          align: "center",
          width: "full",
          children: [
            /* @__PURE__ */ jsxs3(Row3, {
              gap: "sm",
              align: "center",
              children: [
                /* @__PURE__ */ jsx4(Text4, {
                  text: "游戏截图",
                  size: "lg",
                  weight: "semibold",
                  tone: "strong"
                }),
                /* @__PURE__ */ jsx4(Text4, {
                  text: `共 ${images.length} 张`,
                  size: "xs",
                  tone: "muted"
                })
              ]
            }),
            /* @__PURE__ */ jsxs3(Row3, {
              gap: "xs",
              align: "center",
              children: [
                shouldBlurNsfw && sensitiveCount > 0 ? /* @__PURE__ */ jsx4(Button3, {
                  k: "ss-toggle-nsfw",
                  label: revealNsfw ? "隐藏敏感截图" : "显示敏感截图",
                  icon: "eye-outline",
                  variant: "ghost",
                  size: "sm",
                  onPress: () => setRevealNsfw((v) => !v)
                }) : null,
                /* @__PURE__ */ jsx4(Button3, {
                  k: "ss-refresh",
                  label: state.refreshing ? "刷新中…" : "刷新",
                  icon: "refresh",
                  variant: "ghost",
                  size: "sm",
                  loading: state.refreshing,
                  onPress: () => store.loadGame(game, { force: true })
                })
              ]
            })
          ]
        }),
        images.length === 0 ? /* @__PURE__ */ jsx4(Empty, {
          icon: "image-off",
          title: "暂无游戏截图",
          description: "该条目在 Hikarinagi 上暂未收录截图。"
        }) : /* @__PURE__ */ jsx4(Grid, {
          k: "ss-grid",
          cols: 2,
          gap: "md",
          children: images.map((img, idx) => {
            const sensitive = isMediaSensitive(img);
            const blurred = shouldBlurNsfw && sensitive && !revealNsfw && !revealedMap[idx];
            const dimLabel = img.width && img.height ? `${img.width} × ${img.height}` : `#${idx + 1}`;
            return /* @__PURE__ */ jsxs3(Column3, {
              gap: "xs",
              children: [
                /* @__PURE__ */ jsx4(Image2, {
                  k: `ss-img-${idx}`,
                  src: img.url,
                  height: 216,
                  radius: 10,
                  fit: "cover",
                  blur: blurred,
                  previewable: !blurred,
                  onPress: blurred ? () => setRevealedMap((prev) => ({ ...prev, [idx]: true })) : undefined
                }),
                /* @__PURE__ */ jsxs3(Row3, {
                  justify: "between",
                  align: "center",
                  width: "full",
                  children: [
                    /* @__PURE__ */ jsx4(Text4, {
                      text: blurred ? "敏感画面（点击解除模糊）" : `截图 ${idx + 1}`,
                      size: "xs",
                      tone: blurred ? "warning" : "faint"
                    }),
                    /* @__PURE__ */ jsx4(Text4, {
                      text: dimLabel,
                      size: "xs",
                      tone: "faint"
                    })
                  ]
                })
              ]
            }, `ss-item-${idx}`);
          })
        }),
        covers.length > 0 || logos.length > 0 ? /* @__PURE__ */ jsxs3(Fragment3, {
          children: [
            /* @__PURE__ */ jsx4(Divider3, {
              subtle: true
            }),
            /* @__PURE__ */ jsx4(Collapsible2, {
              k: "ss-extra-collapsible",
              title: `封面与透明徽标（封面 ${covers.length} / 徽标 ${logos.length}）`,
              icon: "image-search-outline",
              defaultOpen: false,
              children: /* @__PURE__ */ jsxs3(Column3, {
                gap: "lg",
                width: "full",
                paddingY: "sm",
                children: [
                  logos.length > 0 ? /* @__PURE__ */ jsxs3(Column3, {
                    k: "ss-logos-sec",
                    gap: "sm",
                    width: "full",
                    children: [
                      /* @__PURE__ */ jsx4(Text4, {
                        text: "透明标题徽标",
                        size: "sm",
                        weight: "medium",
                        tone: "strong"
                      }),
                      /* @__PURE__ */ jsx4(Grid, {
                        k: "ss-logos-grid",
                        cols: 2,
                        gap: "sm",
                        children: logos.map((lg, idx) => /* @__PURE__ */ jsxs3(Column3, {
                          gap: "xs",
                          padding: "sm",
                          surface: "subtle",
                          radius: "md",
                          children: [
                            /* @__PURE__ */ jsx4(Image2, {
                              k: `ss-logo-img-${idx}`,
                              src: lg.url,
                              height: 84,
                              radius: 6,
                              fit: "contain",
                              transparent: true,
                              previewable: true
                            }),
                            /* @__PURE__ */ jsx4(Text4, {
                              text: `${formatLangLabel(lg.language) || "默认语言"}${lg.width && lg.height ? ` · ${lg.width}×${lg.height}` : ""}`,
                              size: "xs",
                              tone: "muted"
                            })
                          ]
                        }, `ss-logo-${idx}`))
                      })
                    ]
                  }) : null,
                  covers.length > 0 ? /* @__PURE__ */ jsxs3(Column3, {
                    k: "ss-covers-sec",
                    gap: "sm",
                    width: "full",
                    children: [
                      /* @__PURE__ */ jsx4(Text4, {
                        text: "收录封面",
                        size: "sm",
                        weight: "medium",
                        tone: "strong"
                      }),
                      /* @__PURE__ */ jsx4(Grid, {
                        k: "ss-covers-grid",
                        cols: 3,
                        gap: "sm",
                        children: covers.map((cv, idx) => {
                          const blurred = shouldBlurNsfw && isMediaSensitive(cv) && !revealNsfw;
                          return /* @__PURE__ */ jsxs3(Column3, {
                            gap: "xs",
                            children: [
                              /* @__PURE__ */ jsx4(Image2, {
                                k: `ss-cover-img-${idx}`,
                                src: cv.url,
                                height: 188,
                                radius: 8,
                                fit: "cover",
                                blur: blurred,
                                previewable: !blurred
                              }),
                              /* @__PURE__ */ jsx4(Text4, {
                                text: `${cv.kind || "封面"}${cv.votes ? ` · ${cv.votes} 票` : ""}`,
                                size: "xs",
                                tone: "faint"
                              })
                            ]
                          }, `ss-cover-${idx}`);
                        })
                      })
                    ]
                  }) : null
                ]
              })
            })
          ]
        }) : null
      ]
    });
  };
}

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/db.ts
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
        ctx.logger.error("初始化 Hikarinagi 缓存数据库失败:", err);
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
    if (!row || !row.payload)
      return null;
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
    await ctx.db.exec(`INSERT INTO galgame_cache (hikarinagi_id, payload, fetched_at, updated_at)
       VALUES (?, ?, ?, ?)
       ON CONFLICT(hikarinagi_id) DO UPDATE SET
         payload = excluded.payload,
         fetched_at = excluded.fetched_at,
         updated_at = excluded.updated_at`, [hikarinagiId, JSON.stringify(detail), now, now]);
    return now;
  }
  async function readBinding(localGameId) {
    await ensureDB();
    const row = await ctx.db.queryOne("SELECT local_game_id, hikarinagi_id, manual, matched_title, updated_at FROM game_bindings WHERE local_game_id = ?", [localGameId]);
    if (!row)
      return null;
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
    await ctx.db.exec(`INSERT INTO game_bindings (local_game_id, hikarinagi_id, manual, matched_title, updated_at)
       VALUES (?, ?, ?, ?, ?)
       ON CONFLICT(local_game_id) DO UPDATE SET
         hikarinagi_id = excluded.hikarinagi_id,
         manual = excluded.manual,
         matched_title = excluded.matched_title,
         updated_at = excluded.updated_at`, [localGameId, hikarinagiId || "", manual ? 1 : 0, matchedTitle || "", now]);
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

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/store.ts
import { useEffect as useEffect5, useState as useState5 } from "lunabox";
var NEGATIVE_CACHE_MS = 6 * 60 * 60 * 1000;
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
  const entries = new Map;
  const inflight = new Map;
  const listeners = new Map;
  const globalListeners = new Set;
  let configVersion = 0;
  function getConfig() {
    return ctx.config.get() || {};
  }
  function getCacheTtlMs() {
    const hours = Number(getConfig().cacheTtlHours);
    const clamped = Number.isFinite(hours) && hours >= 1 ? hours : 24;
    return clamped * 60 * 60 * 1000;
  }
  function getEntry(gameId) {
    if (!gameId)
      return createEmptyEntry();
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
      set = new Set;
      listeners.set(gameId, set);
    }
    set.add(fn);
    globalListeners.add(fn);
    return () => {
      set.delete(fn);
      if (set.size === 0)
        listeners.delete(gameId);
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
    if (!game || !game.id)
      return Promise.resolve();
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
          ctx.ui.toast("已从 Hikarinagi 同步最新游戏详情", "success");
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        ctx.logger.warn("拉取 Hikarinagi 详情失败:", msg);
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
    if (!game || !game.id)
      return;
    const gameId = String(game.id);
    await db.deleteBinding(gameId);
    entries.delete(gameId);
    ctx.ui.toast("已重置 Hikarinagi 关联，正在重新匹配…", "info");
    await loadGame(game, { force: true });
  }
  function useGameDetail(game) {
    const gameId = game?.id ? String(game.id) : "";
    const [, setTick] = useState5(0);
    useEffect5(() => {
      if (!gameId)
        return;
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

// ../../packages/plugin-sdk/examples/hikarinagi-detail/src/index.tsx
function apply(ctx) {
  ctx.config.define({
    title: "Hikarinagi 详情页重构",
    description: "使用 Hikarinagi Galgame 详情接口重构游戏详情页，并借助独立 SQLite 缓存接口数据。",
    icon: "image-search-outline",
    fields: [
      {
        key: "cacheTtlHours",
        type: "number",
        label: "缓存有效期（小时）",
        description: "超过该时长后打开详情页会自动在后台静默更新缓存。",
        default: 24,
        min: 1,
        max: 720,
        step: 1
      },
      {
        key: "autoSearch",
        type: "boolean",
        label: "非 Hikarinagi 来源自动按名称匹配",
        description: "当游戏来自 VNDB / Bangumi / Steam 等其他元数据来源时，自动搜索并关联 Hikarinagi 条目。",
        default: true
      },
      {
        key: "preferredLogoLang",
        type: "enum",
        label: "优先徽标语言",
        default: "auto",
        options: [
          { value: "auto", label: "跟随应用语言" },
          { value: "zh", label: "优先中文徽标" },
          { value: "ja", label: "优先日文徽标" },
          { value: "en", label: "优先英文徽标" }
        ]
      },
      {
        key: "heroHeight",
        type: "number",
        label: "头图区域高度（px）",
        description: "控制顶部全出血头图横幅的最小高度，推荐 440–500px。",
        default: 460,
        min: 320,
        max: 580,
        step: 10
      },
      {
        key: "blurHeroBanner",
        type: "boolean",
        label: "模糊头图背景",
        description: "默认清晰展示原始高清头图；开启后对所有头图施加高斯模糊。",
        default: false
      },
      {
        key: "accessToken",
        type: "secret",
        label: "自定义 Hikarinagi Token（可选）",
        description: "留空时自动使用 LunaBox 已登录账号或应用内置的 Hikarinagi 访问令牌。",
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
    title: "概览",
    icon: "file-document-outline",
    order: -10,
    render: createOverviewTab(store)
  });
  ctx.ui.slot("game.detail.tab", {
    id: "screenshots",
    title: "游戏截图",
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
