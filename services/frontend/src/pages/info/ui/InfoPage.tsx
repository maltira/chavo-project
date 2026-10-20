import { useNavigate } from 'react-router'

import { ROUTES } from '@/shared/config'
import { Button } from '@/shared/ui'
import { AuthCard, AuthLayout } from '@/widgets/auth-layout'

import styles from './InfoPage.module.scss'

interface Props {
    title: string
}

// справочная страница (FAQ, условия, политика): открывается в новой вкладке из меню профиля и регистрации,
// поэтому доступна без входа; текст пока готовится
function InfoPage({ title }: Props) {
    const navigate = useNavigate()
    return (
        <AuthLayout>
            <AuthCard>
                <h1 className={styles.title}>{title}</h1>
                <p className={styles.text}>Раздел готовится и скоро появится здесь.</p>
                <Button onClick={() => navigate(ROUTES.home)}>В приложение</Button>
            </AuthCard>
        </AuthLayout>
    )
}

export const FaqPage = () => <InfoPage title="FAQ и поддержка" />
export const TermsPage = () => <InfoPage title="Условия использования" />
export const PrivacyPage = () => <InfoPage title="Политика конфиденциальности" />
