import { useSystemParts } from '@/entities/message'
import type { Message } from '@/entities/message'
import { useProfileCard } from '@/entities/user'
import { cn } from '@/shared/lib'

import styles from './MessageList.module.scss'

interface Props {
    message: Message
    appear: boolean
    // закреплённое сообщение строки «закрепил», если оно уже в загруженной истории
    quoted?: Message
    onJump: (messageId: string) => void
}

// событие чата посреди истории: строка по центру, без пузыря, меню и галочек;
// имена - ссылки на профили, начало закреплённого сообщения - переход к нему
export function SystemMessage({ message, appear, quoted, onJump }: Props) {
    const parts = useSystemParts(message, quoted)
    const openProfile = useProfileCard((state) => state.open)

    return (
        <p className={cn(styles.system, appear && styles.appear)} data-message-id={message.id}>
            <span className={styles.systemText}>
                {parts.map((part, i) =>
                    part.type === 'user' ? (
                        <button
                            key={i}
                            type="button"
                            className={styles.systemName}
                            aria-label={`Профиль: ${part.name}`}
                            onClick={() => openProfile(part.userId)}
                        >
                            {part.name}
                        </button>
                    ) : part.type === 'quote' && part.messageId ? (
                        <button
                            key={i}
                            type="button"
                            className={styles.systemName}
                            aria-label="Перейти к закреплённому сообщению"
                            onClick={() => onJump(part.messageId)}
                        >
                            {part.text}
                        </button>
                    ) : (
                        part.text
                    ),
                )}
            </span>
        </p>
    )
}
