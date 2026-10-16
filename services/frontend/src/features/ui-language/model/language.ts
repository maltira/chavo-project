import { useState } from 'react'

const STORAGE_KEY = 'chavo:language'

export type Language = 'ru' | 'en'

export const LANGUAGES: { value: Language; label: string }[] = [
    { value: 'ru', label: 'Русский (Russia)' },
    { value: 'en', label: 'English (USA)' },
]

// выбор запоминается на устройстве; переводов пока нет - тексты остаются русскими (этап i18n)
export function getLanguage(): Language {
    try {
        return localStorage.getItem(STORAGE_KEY) === 'en' ? 'en' : 'ru'
    } catch {
        return 'ru'
    }
}

export function useLanguage() {
    const [language, setLanguageState] = useState(getLanguage)

    function setLanguage(next: Language) {
        try {
            localStorage.setItem(STORAGE_KEY, next)
        } catch {
            // выбор просто не запомнится
        }
        setLanguageState(next)
    }

    return { language, setLanguage }
}
