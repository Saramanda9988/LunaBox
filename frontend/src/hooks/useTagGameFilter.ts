import { useCallback, useEffect, useState } from "react";

import { GetGameIDsByTag, SearchTagsInLibrary } from "../../wailsjs/go/service/TagService";

type SelectTagOptions = {
  manual?: boolean;
};

type UseTagGameFilterOptions = {
  onManualTagChange?: () => void;
};

export function useTagGameFilter({ onManualTagChange }: UseTagGameFilterOptions = {}) {
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState<string>("");
  const [tagSuggestions, setTagSuggestions] = useState<string[]>([]);
  const [tagGameIds, setTagGameIds] = useState<Set<string> | null>(null);

  useEffect(() => {
    if (!tagInput) {
      setTagSuggestions([]);
      return;
    }
    SearchTagsInLibrary(tagInput)
      .then((names) => {
        setTagSuggestions(Array.isArray(names) ? names.filter(name => !selectedTags.includes(name)) : []);
      })
      .catch(() => {
        setTagSuggestions([]);
      });
  }, [tagInput, selectedTags]);

  const updateTagGameIds = useCallback(async (tags: string[]) => {
    if (tags.length === 0) {
      setTagGameIds(null);
      return;
    }
    try {
      const allIdsLists = await Promise.all(tags.map(tag => GetGameIDsByTag(tag)));
      if (allIdsLists.length === 0) {
        setTagGameIds(new Set());
        return;
      }
      let intersection = new Set(Array.isArray(allIdsLists[0]) ? allIdsLists[0] : []);
      for (let index = 1; index < allIdsLists.length; index++) {
        const currentSet = new Set(Array.isArray(allIdsLists[index]) ? allIdsLists[index] : []);
        intersection = new Set([...intersection].filter(id => currentSet.has(id)));
      }
      setTagGameIds(intersection);
    }
    catch {
      setTagGameIds(new Set());
    }
  }, []);

  const selectTag = useCallback((tagName: string, options?: SelectTagOptions) => {
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
  }, [onManualTagChange, updateTagGameIds]);

  const removeTag = useCallback((tagName: string) => {
    setSelectedTags((previous) => {
      const next = previous.filter(tag => tag !== tagName);
      void updateTagGameIds(next);
      return next;
    });
    onManualTagChange?.();
  }, [onManualTagChange, updateTagGameIds]);

  const clearTagFilter = useCallback(() => {
    setSelectedTags([]);
    setTagInput("");
    setTagGameIds(null);
    setTagSuggestions([]);
    onManualTagChange?.();
  }, [onManualTagChange]);

  return {
    selectedTags,
    tagInput,
    setTagInput,
    tagSuggestions,
    tagGameIds,
    selectTag,
    removeTag,
    clearTagFilter,
  };
}
