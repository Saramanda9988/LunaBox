import { enums } from "../../src/bindings/models";

export type GameStatusFilter = enums.GameStatus | "";

export const statusOptions: Array<{ label: string; value: GameStatusFilter }>
  = [
    { label: "common.allStatus", value: "" },
    { label: "common.notStarted", value: enums.GameStatus.StatusNotStarted },
    { label: "common.wantToPlay", value: enums.GameStatus.StatusWantToPlay },
    { label: "common.playing", value: enums.GameStatus.StatusPlaying },
    { label: "common.completed", value: enums.GameStatus.StatusCompleted },
    { label: "common.onHold", value: enums.GameStatus.StatusOnHold },
  ];

export const sortOptions: Array<{
  label: string;
  value: enums.GameListSortBy;
}> = [
  { label: "common.name", value: enums.GameListSortBy.GameListSortByName },
  {
    label: "common.company",
    value: enums.GameListSortBy.GameListSortByCompany,
  },
  {
    label: "common.lastPlayedAt",
    value: enums.GameListSortBy.GameListSortByLastPlayedAt,
  },
  {
    label: "common.createdAt",
    value: enums.GameListSortBy.GameListSortByCreatedAt,
  },
  { label: "common.rating", value: enums.GameListSortBy.GameListSortByRating },
  {
    label: "common.releaseDate",
    value: enums.GameListSortBy.GameListSortByReleaseDate,
  },
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
