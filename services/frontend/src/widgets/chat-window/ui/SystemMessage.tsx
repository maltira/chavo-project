import { useSystemParts } from '@/entities/message'
import type { Message } from '@/entities/message'
import { useProfileCard } from '@/entities/user'
import { cn } from '@/shared/lib'

import styles from './MessageList.module.scss'

// событие состава группы посреди истории: строка по центру, без пузыря, меню и галочек;
// имена - ссылки на профили
export function SystemMessage({ message, appear }: { message: Message; appear: boolean }) {
    const parts = useSystemParts(message)
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
                    ) : (
                        part.text
                    ),
                )}
            </span>
        </p>
    )
}
