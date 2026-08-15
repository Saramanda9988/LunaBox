import { useCallback, useEffect, useState } from "react";

import {
  FilterExistingTagNames,
  GetGameIDsByTag,
  SearchTagsInLibrary,
} from "../../bindings/lunabox/internal/service/tagservice";
import {
  filterTagNamesByDisplayQuery,
  findRawTagNamesByTranslatedQuery,
} from "../utils/tagTranslation";

type SelectTagOptions = {
  manual?: boolean;
};

type UseTagGameFilterOptions = {
  enableTagTranslation?: boolean;
  initialSelectedTags?: string[];
  onManualTagChange?: () => void;
};

export function useTagGameFilter({
  enableTagTranslation = true,
  initialSelectedTags = [],
  onManualTagChange,
}: UseTagGameFilterOptions = {}) {
  const [selectedTags, setSelectedTags] = useState<string[]>(
    () => initialSelectedTags,
  );
  const [tagInput, setTagInput] = useState<string>("");
  const [tagSuggestions, setTagSuggestions] = useState<string[]>([]);
  const [tagGameIds, setTagGameIds] = useState<Set<string> | null>(null);

  useEffect(() => {
    const normalizedInput = tagInput.trim();
    if (!normalizedInput) {
      setTagSuggestions([]);
      return;
    }

    let cancelled = false;
    const loadTagSuggestions = async () => {
      try {
        const names = await SearchTagsInLibrary(normalizedInput);
        const rawNames = Array.isArray(names) ? names : [];
        const translatedMatches = enableTagTranslation
          ? findRawTagNamesByTranslatedQuery(normalizedInput)
          : [];
        const existingTranslatedMatches
          = translatedMatches.length > 0
            ? await FilterExistingTagNames(translatedMatches)
            : [];
        if (cancelled) {
          return;
        }
        const mergedNames = [
          ...new Set([...rawNames, ...existingTranslatedMatches]),
        ];
        setTagSuggestions(
          filterTagNamesByDisplayQuery(
            mergedNames,
            normalizedInput,
            enableTagTranslation,
          )
            .filter(name => !selectedTags.includes(name))
            .slice(0, 50),
        );
      }
      catch {
        if (!cancelled) {
          setTagSuggestions([]);
        }
      }
    };

    void loadTagSuggestions();
    return () => {
      cancelled = true;
    };
  }, [enableTagTranslation, tagInput, selectedTags]);

  const updateTagGameIds = useCallback(async (tags: string[]) => {
    if (tags.length === 0) {
      setTagGameIds(null);
      return;
    }
    try {
      const allIdsLists = await Promise.all(
        tags.map(tag => GetGameIDsByTag(tag)),
      );
      if (allIdsLists.length === 0) {
        setTagGameIds(new Set());
        return;
      }
      let intersection = new Set(
        Array.isArray(allIdsLists[0]) ? allIdsLists[0] : [],
      );
      for (let index = 1; index < allIdsLists.length; index++) {
        const currentSet = new Set(
          Array.isArray(allIdsLists[index]) ? allIdsLists[index] : [],
        );
        intersection = new Set(
          [...intersection].filter(id => currentSet.has(id)),
        );
      }
      setTagGameIds(intersection);
    }
    catch {
      setTagGameIds(new Set());
    }
  }, []);

  const selectTag = useCallback(
    (tagName: string, options?: SelectTagOptions) => {
      const normalizedName = tagName.trim();
      if (!normalizedName) {
        return;
      }
      setSelectedTags((previous) => {
        if (previous.includes(normalizedName)) {
          return previous;
        }
        const next = [...previous, normalizedName];
        void updateTagGameIds(next);
        return next;
      });
      setTagInput("");
      if (options?.manual !== false) {
        onManualTagChange?.();
      }
    },
    [onManualTagChange, updateTagGameIds],
  );

  const removeTag = useCallback(
    (tagName: string) => {
      setSelectedTags((previous) => {
        const next = previous.filter(tag => tag !== tagName);
        void updateTagGameIds(next);
        return next;
      });
      onManualTagChange?.();
    },
    [onManualTagChange, updateTagGameIds],
  );

  const clearTagFilter = useCallback(() => {
    setSelectedTags([]);
    setTagInput("");
    setTagGameIds(null);
    setTagSuggestions([]);
    onManualTagChange?.();
  }, [onManualTagChange]);

  const replaceSelectedTags = useCallback(
    (tags: string[]) => {
      const next = [...new Set(tags.map(tag => tag.trim()).filter(Boolean))];
      setSelectedTags(next);
      setTagInput("");
      setTagSuggestions([]);
      void updateTagGameIds(next);
      onManualTagChange?.();
    },
    [onManualTagChange, updateTagGameIds],
  );

  return {
    selectedTags,
    tagInput,
    setTagInput,
    tagSuggestions,
    tagGameIds,
    selectTag,
    removeTag,
    clearTagFilter,
    replaceSelectedTags,
  };
}
