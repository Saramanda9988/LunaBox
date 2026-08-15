import type {
  enums,
  metadata,
  models,
  vo,
} from "../../../../src/bindings/models";

export type ImportMatchStatus
  = | "pending"
    | "matched"
    | "not_found"
    | "error"
    | "manual";

export type ImportCandidate = {
  folderPath: string;
  gameDirectory: string;
  folderName: string;
  executables: string[];
  selectedExe: string;
  searchName: string;
  sourceType?: enums.SourceType;
  sourceId?: string;
  sizeOnDisk?: number;
  isSelected: boolean;
  importStatus: string;
  skipReason: string;
  existingName: string;
  matchedGame: models.Game | null;
  matchedTags: metadata.TagItem[];
  matchSource: enums.SourceType | null;
  matchStatus: ImportMatchStatus;
  matchError?: string;
  metadataDuplicateExistingId?: string;
  metadataDuplicateExistingName?: string;
  allMatches?: vo.GameMetadataFromWebVO[];
};

export type MatchProgressState = {
  current: number;
  total: number;
  gameName: string;
};
