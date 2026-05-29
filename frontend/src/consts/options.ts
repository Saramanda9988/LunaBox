import { enums } from "../../wailsjs/go/models";

export type GameStatusFilter = enums.GameStatus | "";

export const statusOptions: Array<{ label: string; value: GameStatusFilter }>
  = [
    { label: "common.allStatus", value: "" },
    { label: "common.notStarted", value: enums.GameStatus.NOT_STARTED },
    { label: "common.wantToPlay", value: enums.GameStatus.WANT_TO_PLAY },
    { label: "common.playing", value: enums.GameStatus.PLAYING },
    { label: "common.completed", value: enums.GameStatus.COMPLETED },
    { label: "common.onHold", value: enums.GameStatus.ON_HOLD },
  ];

export const sortOptions: Array<{
  label: string;
  value: enums.GameListSortBy;
}> = [
  { label: "common.name", value: enums.GameListSortBy.NAME },
  { label: "common.lastPlayedAt", value: enums.GameListSortBy.LAST_PLAYED_AT },
  { label: "common.createdAt", value: enums.GameListSortBy.CREATED_AT },
  { label: "common.rating", value: enums.GameListSortBy.RATING },
  { label: "common.releaseDate", value: enums.GameListSortBy.RELEASE_DATE },
];

export const APP_ZOOM_LEVELS = [0.8, 0.9, 1, 1.1, 1.25, 1.5] as const;
export const DEFAULT_APP_ZOOM = 1;
type AppZoomLevel = (typeof APP_ZOOM_LEVELS)[number];

export const appZoomOptions = APP_ZOOM_LEVELS.map(value => ({
  label: `${Math.round(value * 100)}%`,
  value: String(value),
}));

export const languageOptions = [
  { value: "zh-CN", label: "简体中文" },
  { value: "zh-TW", label: "繁體中文" },
  { value: "en-US", label: "English" },
  { value: "ja-JP", label: "日本語" },
];

export function normalizeAppZoomFactor(value?: number) {
  if (typeof value !== "number" || Number.isNaN(value) || value <= 0) {
    return DEFAULT_APP_ZOOM;
  }

  let nearest: AppZoomLevel = APP_ZOOM_LEVELS[0];
  let nearestDistance = Math.abs(value - nearest);

  for (const zoomLevel of APP_ZOOM_LEVELS) {
    const distance = Math.abs(value - zoomLevel);
    if (distance < nearestDistance) {
      nearest = zoomLevel;
      nearestDistance = distance;
    }
  }

  return nearest;
}

export function getNextAppZoomFactor(current: number, direction: 1 | -1) {
  const normalized = normalizeAppZoomFactor(current);
  const currentIndex = APP_ZOOM_LEVELS.findIndex(
    level => level === normalized,
  );
  const nextIndex = Math.min(
    APP_ZOOM_LEVELS.length - 1,
    Math.max(0, currentIndex + direction),
  );
  return APP_ZOOM_LEVELS[nextIndex];
}
