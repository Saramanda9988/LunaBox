import { enums } from "../../wailsjs/go/models";

export const statusOptions = [
  { label: "common.allStatus", value: "" },
  { label: "common.notStarted", value: enums.GameStatus.NOT_STARTED },
  { label: "common.playing", value: enums.GameStatus.PLAYING },
  { label: "common.completed", value: enums.GameStatus.COMPLETED },
  { label: "common.onHold", value: enums.GameStatus.ON_HOLD },
];

export const sortOptions = [
  { label: "common.name", value: "name" },
  { label: "common.createdAt", value: "created_at" },
];
