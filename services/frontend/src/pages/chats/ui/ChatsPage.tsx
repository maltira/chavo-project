import { useParams } from 'react-router'

import { Icon } from '@/shared/ui'
import { AppShell } from '@/widgets/app-shell'
import { ChatList } from '@/widgets/chat-list'
import { ChatWindow } from '@/widgets/chat-window'

import styles from './ChatsPage.module.scss'

// главная приложения: / - пустой чат, /chats/:id - открытый чат
export function ChatsPage() {
    const { id } = useParams()

    return (
        <AppShell list={(compact) => <ChatList compact={compact} />}>
            {id ? (
                // key: при переходе в другой чат состояние окна (ответ, правка, текст) сбрасывается
                <ChatWindow key={id} conversationId={id} />
            ) : (
                <div className={styles.placeholder}>
                    <Icon name="shape-message" size={67} color="var(--color-text)" />
                    <p className={styles.text}>Выберите или создайте новый чат</p>
                </div>
            )}
        </AppShell>
    )
}
