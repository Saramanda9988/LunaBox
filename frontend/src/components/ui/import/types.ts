import type { enums, metadata, models, vo } from "../../../../wailsjs/go/models";

export type ImportMatchStatus
  = | "pending"
    | "matched"
    | "not_found"
    | "error"
    | "manual";

export type ImportCandidate = {
  folderPath: string;
  folderName: string;
  executables: string[];
  selectedExe: string;
  searchName: string;
  isSelected: boolean;
  matchedGame: models.Game | null;
  matchedTags: metadata.TagItem[];
  matchSource: enums.SourceType | null;
  matchStatus: ImportMatchStatus;
  allMatches?: vo.GameMetadataFromWebVO[];
};

export type MatchProgressState = {
  current: number;
  total: number;
  gameName: string;
};
