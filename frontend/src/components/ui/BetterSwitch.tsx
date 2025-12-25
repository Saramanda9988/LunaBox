import * as Switch from '@radix-ui/react-switch'

interface BetterSwitchProps {
  id: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  disabled?: boolean
}

export function BetterSwitch({ id, checked, onCheckedChange, disabled }: BetterSwitchProps) {
  return (
    <Switch.Root
      id={id}
      checked={checked}
      onCheckedChange={onCheckedChange}
      disabled={disabled}
      className={`
        w-11 h-6 rounded-full relative outline-none cursor-pointer transition-colors
        ${checked ? 'bg-neutral-600' : 'bg-brand-300 dark:bg-brand-600'}
        ${disabled ? 'opacity-50 cursor-not-allowed' : 'hover:brightness-110'}
        focus:ring-2 focus:ring-neutral-500 focus:ring-offset-2 dark:focus:ring-offset-brand-900
      `}
    >
      <Switch.Thumb
        className={`
          block w-5 h-5 bg-white rounded-full shadow-sm transition-transform duration-100 translate-x-0.5
          ${checked ? 'translate-x-5.5' : 'translate-x-0.5'}
        `}
      />
    </Switch.Root>
  )
}
