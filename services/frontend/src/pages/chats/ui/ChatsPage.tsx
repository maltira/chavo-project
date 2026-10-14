import { Icon } from '@/shared/ui'
import { AppShell } from '@/widgets/app-shell'

import styles from './ChatsPage.module.scss'

// главная приложения; пока чат не выбран, справа подсказка
export function ChatsPage() {
    return (
        <AppShell>
            <div className={styles.placeholder}>
                <Icon name="shape-message" size={67} color="var(--color-text)" />
                <p className={styles.text}>Выберите или создайте новый чат</p>
            </div>
        </AppShell>
    )
}
