import type { enums } from "../../../../src/bindings/models";
import type { ImportCandidate } from "./types";

import { CheckImportMetadataDuplicates } from "../../../../bindings/lunabox/internal/service/importservice";
import { vo } from "../../../../src/bindings/models";

function getMetadataDuplicateKey(
  source: enums.SourceType | null | undefined,
  sourceId: string | undefined,
): string {
  if (!source || !sourceId) {
    return "";
  }
  return `${source}\0${sourceId.trim().toLowerCase()}`;
}

function getCandidateMetadataSourceKeys(candidate: ImportCandidate): string[] {
  const keys: string[] = [];
  const seen = new Set<string>();
  const addKey = (
    source: enums.SourceType | null | undefined,
    sourceId: string | undefined,
  ) => {
    const key = getMetadataDuplicateKey(source, sourceId);
    if (key && !seen.has(key)) {
      seen.add(key);
      keys.push(key);
    }
  };

  addKey(
    candidate.matchSource || candidate.matchedGame?.source_type,
    candidate.matchedGame?.source_id,
  );
  for (const source of candidate.matchedGame?.metadata_sources || []) {
    addKey(source.source_type, source.source_id);
  }
  return keys;
}

export async function applyMetadataDuplicateHints(
  candidates: ImportCandidate[],
): Promise<ImportCandidate[]> {
  const requestsByKey = new Map<string, vo.ImportMetadataDuplicateRequest>();

  for (const candidate of candidates) {
    for (const key of getCandidateMetadataSourceKeys(candidate)) {
      if (requestsByKey.has(key)) {
        continue;
      }
      const separatorIndex = key.indexOf("\0");
      requestsByKey.set(
        key,
        new vo.ImportMetadataDuplicateRequest({
          source: key.slice(0, separatorIndex) as enums.SourceType,
          source_id: key.slice(separatorIndex + 1),
        }),
      );
    }
  }

  if (requestsByKey.size === 0) {
    return candidates.map(candidate => ({
      ...candidate,
      metadataDuplicateExistingId: undefined,
      metadataDuplicateExistingName: undefined,
    }));
  }

  const results = await CheckImportMetadataDuplicates([
    ...requestsByKey.values(),
  ]);
  const resultsByKey = new Map(
    (results || []).map(result => [
      getMetadataDuplicateKey(result.source, result.source_id),
      result,
    ]),
  );

  return candidates.map((candidate) => {
    const duplicate = getCandidateMetadataSourceKeys(candidate)
      .map(key => resultsByKey.get(key))
      .find(result => result?.exists);

    return {
      ...candidate,
      metadataDuplicateExistingId: duplicate?.exists
        ? duplicate.existing_id
        : undefined,
      metadataDuplicateExistingName: duplicate?.exists
        ? duplicate.existing_name
        : undefined,
    };
  });
}
