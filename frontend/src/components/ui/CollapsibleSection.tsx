import { useState, ReactNode } from 'react'

interface CollapsibleSectionProps {
  title: string
  icon: string
  children: ReactNode
  defaultOpen?: boolean
}

export function CollapsibleSection({ title, icon, children, defaultOpen = true }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <div className="bg-brand-50 dark:bg-brand-800 rounded-xl border border-brand-200 dark:border-brand-700 overflow-hidden">
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 hover:bg-brand-300 dark:hover:bg-brand-600 transition-colors"
      >
        <h2 className="text-lg font-semibold text-brand-900 dark:text-white flex items-center gap-2">
          <span className={`${icon} text-xl text-blue-500 dark:text-blue-400`} />
          {title}
        </h2>
        <span className={`i-mdi-chevron-down text-xl text-brand-500 transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`} />
      </button>
      <div className={`transition-all duration-200 ${isOpen ? 'max-h-[2000px] opacity-100' : 'max-h-0 opacity-0 overflow-hidden'}`}>
        <div className="p-5 space-y-4">
          {children}
        </div>
      </div>
    </div>
  )
}
