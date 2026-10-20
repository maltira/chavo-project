import { useState } from 'react'

const STORAGE_KEY = 'chavo:accent'

// цвет по умолчанию - из _themes.scss
export const DEFAULT_ACCENT = '#D97757'
export const ACCENTS = [
    DEFAULT_ACCENT,
    '#E8A33D',
    '#3FB97A',
    '#3A9AD9',
    '#7C6CF2',
    '#C46BD6',
    '#E5607A',
    '#8A8F98',
]

const isHex = (value: string) => /^#[0-9A-F]{6}$/i.test(value)

export function getInitialAccent(): string {
    try {
        const stored = localStorage.getItem(STORAGE_KEY)
        return stored && isHex(stored) ? stored.toUpperCase() : DEFAULT_ACCENT
    } catch {
        return DEFAULT_ACCENT
    }
}

// акцент подменяет переменную темы прямо на <html>: действует только на этом устройстве
export function applyAccent(color: string) {
    // только #RRGGBB: иначе сломается переменная темы, а неверное значение запомнится
    if (!isHex(color)) {
        return
    }
    const root = document.documentElement
    if (color.toUpperCase() === DEFAULT_ACCENT) {
        root.style.removeProperty('--color-accent')
    } else {
        root.style.setProperty('--color-accent', color)
    }
    try {
        localStorage.setItem(STORAGE_KEY, color.toUpperCase())
    } catch {
        // выбор просто не запомнится
    }
}

export function useAccent() {
    const [accent, setAccentState] = useState(getInitialAccent)

    function setAccent(color: string) {
        applyAccent(color)
        setAccentState(color.toUpperCase())
    }

    return { accent, setAccent }
}
