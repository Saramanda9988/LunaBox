import React from 'react'

interface SortOption {
  label: string
  value: string
}

interface FilterBarProps {
  searchQuery: string
  onSearchChange: (value: string) => void
  searchPlaceholder?: string
  sortBy: string
  onSortByChange: (value: string) => void
  sortOptions: SortOption[]
  sortOrder: 'asc' | 'desc'
  onSortOrderChange: (order: 'asc' | 'desc') => void
  actionButton?: React.ReactNode
  extraButtons?: React.ReactNode
}

export function FilterBar({
  searchQuery,
  onSearchChange,
  searchPlaceholder = '搜索...',
  sortBy,
  onSortByChange,
  sortOptions,
  sortOrder,
  onSortOrderChange,
  actionButton,
  extraButtons,
}: FilterBarProps) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 my-4">
      <div className="relative flex-1 max-w-md">
        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
          <div className="i-mdi-magnify text-brand-500" />
        </div>
        <input
          type="text"
          className="block w-auto p-2 pl-10 text-sm text-brand-900 border border-brand-300 rounded-lg bg-white focus:ring-blue-500 focus:border-blue-500 dark:bg-brand-900 dark:border-brand-700 dark:placeholder-brand-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
          placeholder={searchPlaceholder}
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
        />
      </div>

      <div className="flex items-center gap-2">
        <select
          value={sortBy}
          onChange={(e) => onSortByChange(e.target.value)}
          className="bg-white border border-brand-300 text-brand-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block p-2 dark:bg-brand-900 dark:border-brand-600 dark:placeholder-brand-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
        >
          {sortOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>

        <button
          onClick={() => onSortOrderChange(sortOrder === 'asc' ? 'desc' : 'asc')}
          className="p-2 text-brand-500 hover:text-brand-900 dark:text-brand-400 dark:hover:text-white rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700"
          title={sortOrder === 'asc' ? '升序' : '降序'}
        >
          <div className={sortOrder === 'asc' ? "i-mdi-sort-ascending text-xl" : "i-mdi-sort-descending text-xl"} />
        </button>

        {extraButtons}
        {actionButton}
      </div>
    </div>
  )
}
