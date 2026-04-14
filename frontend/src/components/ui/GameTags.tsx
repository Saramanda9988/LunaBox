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
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => setIsAdding(true)}
          className="inline-flex items-center gap-1.5 rounded-full border border-dashed border-brand-300 px-3 py-1.5 text-xs text-brand-500 transition-all duration-200 hover:border-brand-500 hover:bg-brand-50 hover:text-brand-600 dark:border-brand-600 dark:text-brand-400 dark:hover:border-brand-400 dark:hover:bg-brand-800/50 dark:hover:text-brand-200"
        >
          <div className="i-mdi-plus text-sm" />
          {t("tags.add")}
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
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
          className="w-32 rounded-full border border-brand-300 bg-white/90 px-3 py-1.5 text-xs text-brand-900 outline-none transition-all duration-200 placeholder:text-brand-400 focus:border-brand-500 focus:bg-white focus:ring-2 focus:ring-brand-200 dark:border-brand-600 dark:bg-brand-900/80 dark:text-white dark:placeholder:text-brand-500 dark:focus:border-brand-400 dark:focus:bg-brand-900 dark:focus:ring-brand-700"
        />
      ) : (
        <button
          type="button"
          onClick={() => setIsAdding(true)}
          className="inline-flex items-center gap-1.5 rounded-full border border-dashed border-brand-300 px-3 py-1.5 text-xs text-brand-500 transition-all duration-200 hover:border-brand-500 hover:bg-brand-50 hover:text-brand-600 dark:border-brand-600 dark:text-brand-400 dark:hover:border-brand-400 dark:hover:bg-brand-800/50 dark:hover:text-brand-200"
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
  const pillClass = isUser
    ? "border border-dashed border-brand-400/80 bg-white/70 text-brand-700 dark:border-brand-500/70 dark:bg-brand-900/45 dark:text-brand-200"
    : "border border-brand-200/90 bg-brand-50/70 text-brand-700 dark:border-brand-700/80 dark:bg-brand-800/55 dark:text-brand-200";
  const textButtonClass
    = "max-w-full truncate rounded-full px-0.5 transition-colors duration-200";

  return (
    <span
      className={`group relative inline-flex max-w-full items-center rounded-full px-3 py-1.5 text-xs transition-colors duration-200 ${pillClass}`}
    >
      {isSpoiler ? (
        <button
          type="button"
          onClick={() => setRevealed(true)}
          className={`${textButtonClass} cursor-pointer select-none blur-sm hover:text-brand-900 hover:blur-none dark:hover:text-white`}
          title={t("tags.revealSpoiler")}
        >
          {tag.name}
        </button>
      ) : (
        <button
          type="button"
          onClick={onClick}
          className={`${textButtonClass} cursor-pointer hover:text-brand-900 dark:hover:text-white`}
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
          className="absolute -right-1 -top-1 inline-flex h-4.5 w-4.5 items-center justify-center rounded-full border border-brand-200 bg-white text-brand-400 opacity-0 shadow-sm transition-all duration-200 group-hover:translate-y-0 group-hover:scale-100 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:scale-100 group-focus-within:opacity-100 hover:border-red-200 hover:bg-red-50 hover:text-red-500 focus:translate-y-0 focus:scale-100 focus:opacity-100 dark:border-brand-700 dark:bg-brand-950 dark:text-brand-500 dark:hover:border-red-500/40 dark:hover:bg-red-500/12 dark:hover:text-red-300"
        >
          <div className="i-mdi-close text-[10px]" />
        </button>
      )}
    </span>
  );
}
