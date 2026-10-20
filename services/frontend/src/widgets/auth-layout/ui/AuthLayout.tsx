import { useState } from 'react'
import type { ReactNode } from 'react'

import { ThemeToggle } from '@/features/toggle-theme'
import { IconButton, LanguageMenu, Logo } from '@/shared/ui'

import { AuthFooter } from './AuthFooter'
import styles from './AuthLayout.module.scss'

// язык пока заглушка
const LANGUAGES = [
    { value: 'ru', tag: 'RU', label: 'Русский' },
    { value: 'en', tag: 'EN', label: 'Английский' },
]

interface Props {
    children: ReactNode
    showLogo?: boolean // логотип слева сверху
    showFooter?: boolean // подвал с Telegram
}

// фон экранов auth
export function AuthLayout({ children, showLogo = true, showFooter = true }: Props) {
    const [language, setLanguage] = useState('ru')

    return (
        <div className={styles.page}>
            {showLogo && <Logo width={114} className={styles.logo} />}

            <div className={styles.controls}>
                <ThemeToggle />
                <LanguageMenu value={language} options={LANGUAGES} onChange={setLanguage} />
            </div>

            <main className={styles.content}>{children}</main>

            {showFooter && <AuthFooter />}
        </div>
    )
}

interface CardProps {
    children: ReactNode
    onBack?: () => void
}

export function AuthCard({ children, onBack }: CardProps) {
    return (
        <section className={styles.card}>
            {onBack && (
                <IconButton
                    size={40}
                    icon="arrow-left"
                    aria-label="Назад"
                    className={styles.back}
                    onClick={onBack}
                />
            )}
            {children}
        </section>
    )
}
