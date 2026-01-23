import React, { useEffect, useState } from "react";

interface SortOption {
  label: string;
  value: string;
}

interface FilterOption {
  label: string;
  value: string;
}

interface FilterBarProps {
  searchQuery: string;
  onSearchChange: (value: string) => void;
  searchPlaceholder?: string;
  sortBy: string;
  onSortByChange: (value: string) => void;
  sortOptions: SortOption[];
  sortOrder: "asc" | "desc";
  onSortOrderChange: (order: "asc" | "desc") => void;
  // 状态筛选
  statusFilter?: string;
  onStatusFilterChange?: (value: string) => void;
  statusOptions?: FilterOption[];
  actionButton?: React.ReactNode;
  extraButtons?: React.ReactNode;
  // 持久化存储键，传入后会自动保存和恢复排序设置
  storageKey?: string;
}

export function FilterBar({
  searchQuery,
  onSearchChange,
  searchPlaceholder = "搜索...",
  sortBy,
  onSortByChange,
  sortOptions,
  sortOrder,
  onSortOrderChange,
  statusFilter,
  onStatusFilterChange,
  statusOptions,
  actionButton,
  extraButtons,
  storageKey,
}: FilterBarProps) {
  const [initialized, setInitialized] = useState(false);

  // 初始化时从 localStorage 恢复排序设置
  useEffect(() => {
    if (storageKey && !initialized) {
      const savedSortBy = localStorage.getItem(`${storageKey}_sortBy`);
      const savedSortOrder = localStorage.getItem(`${storageKey}_sortOrder`);

      // 验证保存的 sortBy 是否在 sortOptions 中
      if (savedSortBy && sortOptions.some(opt => opt.value === savedSortBy)) {
        onSortByChange(savedSortBy);
      }

      if (savedSortOrder === "asc" || savedSortOrder === "desc") {
        onSortOrderChange(savedSortOrder);
      }

      setInitialized(true);
    }
  }, [storageKey, sortOptions, initialized]);

  // 处理排序方式变更
  const handleSortByChange = (value: string) => {
    onSortByChange(value);
    if (storageKey) {
      localStorage.setItem(`${storageKey}_sortBy`, value);
    }
  };

  // 处理排序顺序变更
  const handleSortOrderChange = (order: "asc" | "desc") => {
    onSortOrderChange(order);
    if (storageKey) {
      localStorage.setItem(`${storageKey}_sortOrder`, order);
    }
  };
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 my-4">
      <div className="relative flex-1 max-w-md">
        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
          <div className="i-mdi-magnify text-brand-500" />
        </div>
        <input
          type="text"
          className="glass-input block w-auto p-2 pl-10 text-sm text-brand-900 dark:text-white
                     bg-white dark:bg-brand-900
                     border border-brand-300 dark:border-brand-700
                     rounded-lg
                     placeholder:text-brand-400 dark:placeholder:text-brand-400
                     focus:ring-neutral-500 focus:border-neutral-500
                     dark:focus:ring-neutral-500 dark:focus:border-neutral-500"
          placeholder={searchPlaceholder}
          value={searchQuery}
          onChange={e => onSearchChange(e.target.value)}
        />
      </div>

      <div className="flex items-center gap-2">
        {/* 状态筛选 */}
        {statusOptions && onStatusFilterChange && (
          <select
            value={statusFilter || ""}
            onChange={e => onStatusFilterChange(e.target.value)}
            className="glass-input
                       bg-white dark:bg-brand-900
                       border border-brand-300 dark:border-brand-600
                       text-brand-900 dark:text-white
                       placeholder:text-brand-400 dark:placeholder:text-brand-400
                       text-sm rounded-lg
                       focus:ring-neutral-500 focus:border-neutral-500
                       dark:focus:ring-neutral-500 dark:focus:border-neutral-500
                       block p-2"
          >
            {statusOptions.map(option => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        )}

        <select
          value={sortBy}
          onChange={e => handleSortByChange(e.target.value)}
          className="glass-input
                     bg-white dark:bg-brand-900
                     border border-brand-300 dark:border-brand-600
                     text-brand-900 dark:text-white
                     placeholder:text-brand-400 dark:placeholder:text-brand-400
                     text-sm rounded-lg
                     focus:ring-neutral-500 focus:border-neutral-500
                     dark:focus:ring-neutral-500 dark:focus:border-neutral-500
                     block p-2"
        >
          {sortOptions.map(option => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>

        <button
          onClick={() => handleSortOrderChange(sortOrder === "asc" ? "desc" : "asc")}
          className="glass-panel p-2
                     text-brand-500 dark:text-brand-400
                     hover:text-brand-900 dark:hover:text-white
                     bg-white dark:bg-brand-800
                     border border-brand-200 dark:border-brand-700
                     rounded-lg
                     hover:bg-brand-100 dark:hover:bg-brand-700"
          title={sortOrder === "asc" ? "升序" : "降序"}
        >
          <div className={sortOrder === "asc" ? "i-mdi-sort-ascending text-xl" : "i-mdi-sort-descending text-xl"} />
        </button>

        {extraButtons}
        {actionButton}
      </div>
    </div>
  );
}
