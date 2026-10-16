import { useState } from 'react'
import { useParams } from 'react-router'

import { CreateGroupModal } from '@/features/create-group'

import { Icon } from '@/shared/ui'
import { AppShell } from '@/widgets/app-shell'
import { ChatList } from '@/widgets/chat-list'
import { ChatWindow, NewChatWindow } from '@/widgets/chat-window'
import { GroupPreviewModal } from '@/widgets/group-preview'
import { SettingsModal } from '@/widgets/settings'
import { SearchResults } from '@/widgets/sidebar-search'
import { UserProfileModal } from '@/widgets/user-profile'

import styles from './ChatsPage.module.scss'

// главная приложения: / - пустой чат, /chats/:id - открытый чат,
// /chats/new/:userId - личный чат, который создаст первое сообщение
export function ChatsPage() {
    const { id, userId } = useParams()
    const [creating, setCreating] = useState(false)
    const [settings, setSettings] = useState(false)

    return (
        <AppShell
            list={(compact) => <ChatList compact={compact} />}
            search={({ query, close }) => <SearchResults query={query} close={close} />}
            onCreate={() => setCreating(true)}
            onSettings={() => setSettings(true)}
        >
            {id ? (
                // key: при переходе в другой чат состояние окна (ответ, правка, текст) сбрасывается
                <ChatWindow key={id} conversationId={id} />
            ) : userId ? (
                <NewChatWindow key={userId} userId={userId} />
            ) : (
                <div className={styles.placeholder}>
                    <Icon name="shape-message" size={67} color="var(--color-text)" />
                    <p className={styles.text}>Выберите или создайте новый чат</p>
                </div>
            )}
            {/* модалки открываются из разных мест через сторы сущностей, показываются здесь */}
            <UserProfileModal />
            <GroupPreviewModal />
            <CreateGroupModal open={creating} onClose={() => setCreating(false)} />
            <SettingsModal open={settings} onClose={() => setSettings(false)} />
        </AppShell>
    )
}
