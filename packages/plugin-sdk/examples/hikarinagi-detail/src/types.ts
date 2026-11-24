import type { Game } from 'lunabox';

export interface HikarinagiLogo {
  url: string;
  language?: string;
  width?: number;
  height?: number;
}

export interface HikarinagiMedia {
  url: string;
  width?: number;
  height?: number;
  sexual?: number;
  violence?: number;
  votes?: number;
  kind?: string;
}

export interface HikarinagiPrice {
  version?: string;
  amount?: number;
  currency?: string;
  tax_included?: boolean;
}

export interface HikarinagiRating {
  score?: number;
  total?: number;
  played?: number;
  count?: Record<string, number>;
}

export interface HikarinagiTag {
  name: string;
  likes?: number;
}

export interface HikarinagiSteamApp {
  app_id: number;
}

export interface HikarinagiExternalSource {
  bangumi?: string | number;
  vndb?: string;
}

export interface HikarinagiExternalLink {
  label?: string;
  name?: string;
  url: string;
}

export interface HikarinagiGalgameDetail {
  id: number;
  origin_title?: string;
  trans_title?: string;
  en_title?: string;
  aliases?: string[];
  origin_intro?: string;
  trans_intro?: string;
  developer?: string;
  publisher?: string;
  release_date?: string;
  release_date_precision?: string;
  release_date_tbd?: boolean;
  release_date_tbd_note?: string;
  release_status?: string;
  dev_status?: string;
  adv_type?: string;
  origin_lang?: string;
  languages?: string[];
  engine?: string;
  nsfw?: boolean;
  platforms?: string[];
  prices?: HikarinagiPrice[];
  rating?: HikarinagiRating;
  logos?: HikarinagiLogo[];
  images?: HikarinagiMedia[];
  covers?: HikarinagiMedia[];
  tags?: HikarinagiTag[];
  homepage?: string;
  steam_apps?: HikarinagiSteamApp[];
  external_source?: HikarinagiExternalSource;
  external_links?: HikarinagiExternalLink[];
}

export interface HikarinagiSearchHit {
  type?: string;
  id: number;
  title?: string;
  subtitle?: string;
  developer?: string;
}

export interface PluginConfig {
  cacheTtlHours?: number;
  autoSearch?: boolean;
  preferredLogoLang?: 'auto' | 'zh' | 'ja' | 'en';
  heroHeight?: number;
  blurHeroBanner?: boolean;
  accessToken?: string;
}

export type DetailLoadStatus = 'idle' | 'loading' | 'ready' | 'unmatched' | 'error';

export interface DetailEntryState {
  status: DetailLoadStatus;
  hikarinagiId: string;
  manualBound: boolean;
  detail: HikarinagiGalgameDetail | null;
  fetchedAt: number;
  fromCache: boolean;
  refreshing: boolean;
  error: string;
}

export interface SlotRenderProps {
  gameId: string;
  game: Game;
  blurNsfw?: boolean;
  contentMaxWidth?: number;
  /** 宿主隐藏顶栏时的顶部安全区，透传给 HeroBanner */
  safeTop?: number;
}

export interface BannerPick {
  url: string;
  sensitive: boolean;
  fromCover: boolean;
}
