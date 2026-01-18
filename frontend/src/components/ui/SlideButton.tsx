import { ReactNode } from 'react'

export interface SlideButtonOption<T extends string = string> {
  label: string
  value: T
  icon?: ReactNode
}

export interface SlideButtonProps<T extends string = string> {
  options: SlideButtonOption<T>[]
  value: T
  onChange: (value: T) => void
  className?: string
  disabled?: boolean
}

export function SlideButton<T extends string = string>({
  options,
  value,
  onChange,
  className = '',
  disabled = false,
}: SlideButtonProps<T>) {
  return (
    <div className={`flex space-x-2 bg-brand-100 dark:bg-brand-800 p-1 rounded-lg ${className}`}>
      {options.map((option) => (
        <button
          key={option.value}
          onClick={() => !disabled && onChange(option.value)}
          disabled={disabled}
          className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
            value === option.value
              ? 'bg-white dark:bg-brand-700 text-neutral-600 dark:text-neutral-400 shadow-sm'
              : 'text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200'
          } ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
        >
          {option.icon}
          {option.label}
        </button>
      ))}
    </div>
  )
}
