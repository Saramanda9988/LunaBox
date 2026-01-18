import { ReactNode, useEffect, useRef, useState } from 'react'

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
  const [sliderStyle, setSliderStyle] = useState({ width: 0, left: 0 })
  const buttonsRef = useRef<(HTMLButtonElement | null)[]>([])
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const updateSliderPosition = () => {
      const selectedIndex = options.findIndex((opt) => opt.value === value)
      if (selectedIndex !== -1 && buttonsRef.current[selectedIndex]) {
        const button = buttonsRef.current[selectedIndex]
        const container = containerRef.current
        if (button && container) {
          const containerRect = container.getBoundingClientRect()
          const buttonRect = button.getBoundingClientRect()
          setSliderStyle({
            width: buttonRect.width,
            left: buttonRect.left - containerRect.left,
          })
        }
      }
    }

    updateSliderPosition()
    // Also update on resize to handle dynamic changes
    window.addEventListener('resize', updateSliderPosition)
    return () => window.removeEventListener('resize', updateSliderPosition)
  }, [value, options])

  return (
    <div 
      ref={containerRef}
      className={`relative flex bg-brand-100 dark:bg-brand-800 p-1 rounded-lg ${className}`}
    >
      {/* Sliding background */}
      <div
        className="absolute inset-y-1 bg-white dark:bg-brand-700 rounded-md shadow-sm transition-all duration-300 ease-out pointer-events-none"
        style={{
          width: `${sliderStyle.width}px`,
          left: `${sliderStyle.left}px`,
        }}
      />
      
      {/* Buttons */}
      {options.map((option, index) => (
        <button
          key={option.value}
          ref={(el) => (buttonsRef.current[index] = el)}
          onClick={() => !disabled && onChange(option.value)}
          disabled={disabled}
          className={`relative z-10 px-4 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
            value === option.value
              ? 'text-neutral-600 dark:text-neutral-400'
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
