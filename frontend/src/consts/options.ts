import { GameStatus } from "../../bindings/lunabox/internal/enums";

export const statusOptions = [
  { label: "全部状态", value: "" },
  { label: "未开始", value: GameStatus.StatusNotStarted },
  { label: "游玩中", value: GameStatus.StatusPlaying },
  { label: "已通关", value: GameStatus.StatusCompleted },
  { label: "搁置", value: GameStatus.StatusOnHold },
];

export const sortOptions = [
  { label: "名称", value: "name" },
  { label: "添加时间", value: "created_at" },
];
