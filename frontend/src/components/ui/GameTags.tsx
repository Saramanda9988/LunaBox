import type { models } from "../../../wailsjs/go/models";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddUserTag,
  DeleteTag,
  GetTagsByGame,
} from "../../../wailsjs/go/service/TagService";

interface GameTagsProps {
  gameId: string;
  showNSFW?: boolean;
  refreshToken?: number;
}

export function GameTags({
  gameId,
  showNSFW = false,
  refreshToken = 0,
}: GameTagsProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [tags, setTags] = useState<models.GameTag[]>([]);
  const [isAdding, setIsAdding] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    GetTagsByGame(gameId)
      .then(result => setTags(result ?? []))
      .catch(() => {});
  }, [gameId, refreshToken]);

  useEffect(() => {
    if (isAdding) {
      inputRef.current?.focus();
    }
  }, [isAdding]);

  const handleAddTag = async () => {
    const name = inputValue.trim();
    if (!name) {
      setIsAdding(false);
      return;
    }
    try {
      await AddUserTag(gameId, name);
      const updated = await GetTagsByGame(gameId);
      setTags(updated ?? []);
      setInputValue("");
      setIsAdding(false);
    }
    catch {
      toast.error(t("tags.addFailed"));
    }
  };

  const handleDeleteTag = async (tag: models.GameTag) => {
    try {
      await DeleteTag(tag.id);
      setTags(prev => prev.filter(t => t.id !== tag.id));
      if (tag.source !== "user") {
        toast.success(t("tags.deleteScrapedHint"));
      }
    }
    catch {
      toast.error(t("tags.deleteFailed"));
    }
  };

  const handleTagClick = (tagName: string) => {
    navigate({ to: "/library", search: { tagFilter: tagName } });
  };

  const visibleTags = showNSFW ? tags : tags;

  if (visibleTags.length === 0 && !isAdding) {
    return (
      <div className="flex items-center gap-2 flex-wrap">
        <button
          type="button"
          onClick={() => setIsAdding(true)}
          className="flex items-center gap-1 px-2.5 py-1 text-xs rounded-full border border-dashed border-brand-300 dark:border-brand-600 text-brand-500 dark:text-brand-400 hover:border-brand-500 dark:hover:border-brand-400 transition-colors"
        >
          <div className="i-mdi-plus text-sm" />
          {t("tags.add")}
        </button>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      {visibleTags.map(tag => (
        <TagPill
          key={tag.id}
          tag={tag}
          onClick={() => handleTagClick(tag.name)}
          onDelete={() => handleDeleteTag(tag)}
        />
      ))}

      {isAdding ? (
        <input
          ref={inputRef}
          type="text"
          value={inputValue}
          onChange={e => setInputValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter")
              handleAddTag();
            if (e.key === "Escape") {
              setIsAdding(false);
              setInputValue("");
            }
          }}
          onBlur={handleAddTag}
          placeholder={t("tags.inputPlaceholder")}
          className="px-2.5 py-1 text-xs rounded-full border border-brand-400 dark:border-brand-500 bg-white dark:bg-brand-800 text-brand-900 dark:text-white outline-none w-28"
        />
      ) : (
        <button
          type="button"
          onClick={() => setIsAdding(true)}
          className="flex items-center gap-1 px-2.5 py-1 text-xs rounded-full border border-dashed border-brand-300 dark:border-brand-600 text-brand-500 dark:text-brand-400 hover:border-brand-500 dark:hover:border-brand-400 transition-colors"
        >
          <div className="i-mdi-plus text-sm" />
          {t("tags.add")}
        </button>
      )}
    </div>
  );
}

interface TagPillProps {
  tag: models.GameTag;
  onClick: () => void;
  onDelete?: () => void;
}

function TagPill({ tag, onClick, onDelete }: TagPillProps) {
  const { t } = useTranslation();
  const [revealed, setRevealed] = useState(false);
  const isSpoiler = tag.is_spoiler && !revealed;
  const isUser = tag.source === "user";

  return (
    <span
      className={`group relative inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded-full transition-all
        ${
    isUser
      ? "border border-dashed border-brand-400 dark:border-brand-500 text-brand-700 dark:text-brand-300"
      : "border border-brand-200 dark:border-brand-700 text-brand-700 dark:text-brand-300 bg-brand-50 dark:bg-brand-800/60"
    }
      `}
    >
      {isSpoiler ? (
        <button
          type="button"
          onClick={() => setRevealed(true)}
          className="blur-sm hover:blur-none transition-all cursor-pointer select-none"
          title={t("tags.revealSpoiler")}
        >
          {tag.name}
        </button>
      ) : (
        <button
          type="button"
          onClick={onClick}
          className="cursor-pointer hover:underline"
        >
          {tag.name}
        </button>
      )}
      {onDelete && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            if (onDelete) {
              onDelete();
            }
          }}
          className="opacity-0 group-hover:opacity-100 transition-opacity ml-0.5 text-brand-400 hover:text-red-500 dark:hover:text-red-400"
        >
          <div className="i-mdi-close text-xs" />
        </button>
      )}
    </span>
  );
}
