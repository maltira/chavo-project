import { ProfileForm } from '@/features/profile-create'
import { AuthLayout } from '@/widgets/auth-layout'

import styles from './ProfileSetupPage.module.scss'

// первый вход: без профиля API отвечает PROFILE_REQUIRED, поэтому сюда ведёт гард
export function ProfileSetupPage() {
    return (
        <AuthLayout showFooter={false}>
            <div className={styles.layout}>
                <div className={styles.intro}>
                    <h1 className={styles.title}>Познакомимся...</h1>
                    <p className={styles.text}>
                        Перед тем как продолжить, заполни информацию о себе, чтобы другие
                        пользователи понимали, с кем имеют дело
                    </p>
                </div>
                <section className={styles.card}>
                    <ProfileForm />
                </section>
            </div>
        </AuthLayout>
    )
}
