import { useCallback, useState } from 'react'

// Light/dark theme: saved per browser, dark by default.

export type Theme = 'light' | 'dark'

const storageKey = 'govai-theme'

function initialTheme(): Theme {
  try {
    const saved = localStorage.getItem(storageKey)
    if (saved === 'light' || saved === 'dark') return saved
  } catch { /* storage can be blocked; fall back to the default */ }
  return 'dark'
}

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
}

// Applied on load so the first paint already uses the saved theme.
applyTheme(initialTheme())

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')
  const toggle = useCallback(() => {
    setTheme(previous => {
      const next = previous === 'dark' ? 'light' : 'dark'
      applyTheme(next)
      try { localStorage.setItem(storageKey, next) } catch { /* not saved; still applies for this visit */ }
      return next
    })
  }, [])
  return { theme, toggle }
}
