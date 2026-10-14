import { Icon } from '@/shared/ui'
import { AppShell } from '@/widgets/app-shell'
import { ChatList } from '@/widgets/chat-list'

import styles from './ChatsPage.module.scss'

// главная приложения (/ и /chats/:id\)
export function ChatsPage() {
    return (
        <AppShell list={(compact) => <ChatList compact={compact} />}>
            <div className={styles.placeholder}>
                <Icon name="shape-message" size={67} color="var(--color-text)" />
                <p className={styles.text}>Выберите или создайте новый чат</p>
            </div>
        </AppShell>
    )
}
