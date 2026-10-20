import { useState } from 'react'

export type Theme = 'dark' | 'light'

const STORAGE_KEY = 'chavo:theme'

export function getInitialTheme(): Theme {
    try {
        return localStorage.getItem(STORAGE_KEY) === 'light' ? 'light' : 'dark'
    } catch {
        return 'dark'
    }
}

// тема переключается атрибутом data-theme на <html>, цвета берутся из _themes.scss
export function applyTheme(theme: Theme) {
    document.documentElement.dataset.theme = theme
    try {
        localStorage.setItem(STORAGE_KEY, theme)
    } catch {
        // выбор просто не запомнится
    }
}

function currentTheme(): Theme {
    return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark'
}

export function useTheme() {
    const [theme, setTheme] = useState<Theme>(currentTheme)

    function toggle() {
        const next: Theme = theme === 'dark' ? 'light' : 'dark'
        applyTheme(next)
        setTheme(next)
    }

    return { theme, toggle }
}
