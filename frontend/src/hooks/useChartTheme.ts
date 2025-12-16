import { useEffect, useState } from 'react'

export function useChartTheme() {
  const [isDark, setIsDark] = useState(document.documentElement.classList.contains('dark'))

  useEffect(() => {
    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (mutation.attributeName === 'class') {
          setIsDark(document.documentElement.classList.contains('dark'))
        }
      })
    })

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })

    return () => observer.disconnect()
  }, [])

  const textColor = isDark ? '#e5e7eb' : '#374151' // brand-200 : brand-700
  const gridColor = isDark ? '#374151' : '#e5e7eb' // brand-700 : brand-200

  return {
    isDark,
    textColor,
    gridColor,
  }
}
