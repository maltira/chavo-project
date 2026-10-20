import type { ReactNode } from 'react'
import { useNavigate } from 'react-router'

import { ROUTES, TELEGRAM_URL } from '@/shared/config'
import { Button, Segmented, TextLink } from '@/shared/ui'
import { AuthLayout } from '@/widgets/auth-layout'

import { PRIVACY, TERMS, UPDATED_AT } from '../model/content'
import { FaqList } from './FaqList'
import styles from './InfoPage.module.scss'
import { LegalDocument } from './LegalDocument'

type Section = 'faq' | 'terms' | 'privacy'

const PATHS: Record<Section, string> = {
    faq: ROUTES.faq,
    terms: ROUTES.terms,
    privacy: ROUTES.privacy,
}

const TITLES: Record<Section, string> = {
    faq: 'FAQ и поддержка',
    terms: 'Условия использования',
    privacy: 'Политика конфиденциальности',
}

interface Props {
    section: Section
    children: ReactNode
}

// справочная страница (FAQ, условия, политика): открывается в новой вкладке из меню профиля и регистрации,
// поэтому доступна без входа; вкладки сверху переключают разделы
function InfoPage({ section, children }: Props) {
    const navigate = useNavigate()

    return (
        <AuthLayout>
            <article className={styles.card}>
                <Segmented
                    aria-label="Раздел"
                    value={section}
                    onChange={(next) => navigate(PATHS[next])}
                    options={[
                        { value: 'faq', label: 'FAQ' },
                        { value: 'terms', label: 'Условия' },
                        { value: 'privacy', label: 'Конфиденциальность' },
                    ]}
                />

                <header className={styles.header}>
                    <h1 className={styles.title}>{TITLES[section]}</h1>
                    {section !== 'faq' && <p className={styles.muted}>Редакция от {UPDATED_AT}</p>}
                </header>

                {children}

                <p className={styles.contact}>
                    Остались вопросы? Напишите нам в{' '}
                    <TextLink href={TELEGRAM_URL} target="_blank" rel="noopener noreferrer">
                        Telegram
                    </TextLink>
                </p>
            </article>
        </AuthLayout>
    )
}

export const FaqPage = () => (
    <InfoPage section="faq">
        <FaqList />
    </InfoPage>
)

export const TermsPage = () => (
    <InfoPage section="terms">
        <LegalDocument sections={TERMS} />
    </InfoPage>
)

export const PrivacyPage = () => (
    <InfoPage section="privacy">
        <LegalDocument sections={PRIVACY} />
    </InfoPage>
)
