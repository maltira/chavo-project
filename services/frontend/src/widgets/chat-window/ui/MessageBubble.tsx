import { useRef, useState } from 'react'

import type { Message } from '@/entities/message'
import { useProfileCard, UserAvatar, useUser } from '@/entities/user'
import { cn, formatTime, notify, useDismiss } from '@/shared/lib'
import { Icon, IconButton } from '@/shared/ui'

import styles from './MessageBubble.module.scss'

interface Props {
    message: Message
    mine: boolean
    read: boolean // прочитано кем-то кроме меня
    showAuthor: boolean // имя автора над пузырём
    // аватар автора слева (чужие сообщения в группе): show - у нижнего в серии, space - место под него
    avatar: 'show' | 'space' | 'none'
    // продолжение серии того же автора: ближе к сообщению над ним
    continued: boolean
    replyTo: Message | null | undefined
    canEdit: boolean
    canDelete: boolean
    highlighted: boolean // подсветка после перехода по цитате
    appear: boolean // пришло при открытом чате: въезжает снизу
    onReply: (message: Message) => void
    onEdit: (message: Message) => void
    onDelete: (message: Message) => void
    onJump: (messageId: string) => void
}

// аватар автора; нажатие открывает его профиль
function AuthorAvatar({ userId }: { userId: string }) {
    const user = useUser(userId)
    const openProfile = useProfileCard((state) => state.open)
    const profile = user.data
    return (
        <button
            type="button"
            className={styles.avatarButton}
            aria-label={profile ? `Профиль: ${profile.display_name}` : 'Профиль'}
            onClick={() => openProfile(userId)}
        >
            <UserAvatar src={profile?.avatar_url} shape={profile?.avatar_shape} size={32} />
        </button>
    )
}

// высота панели действий с отступом от сообщения
const ACTIONS_SPACE = 68

function AuthorName({ userId, className }: { userId: string; className?: string }) {
    const user = useUser(userId)
    return <span className={className}>{user.data?.display_name ?? ' '}</span>
}

// пузырь сообщения
export function MessageBubble({
    message,
    mine,
    read,
    showAuthor,
    avatar,
    continued,
    replyTo,
    canEdit,
    canDelete,
    highlighted,
    appear,
    onReply,
    onEdit,
    onDelete,
    onJump,
}: Props) {
    const [open, setOpen] = useState(false)

    const [above, setAbove] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)
    useDismiss(open, rootRef, () => setOpen(false))

    const deleted = message.is_deleted
    const interactive = !deleted && !message.pending

    function toggle() {
        if (!open) {
            const row = rootRef.current
            const log = row?.closest('[role="log"]')
            if (row && log) {
                const space =
                    log.getBoundingClientRect().bottom - row.getBoundingClientRect().bottom
                setAbove(space < ACTIONS_SPACE)
            }
        }
        setOpen((value) => !value)
    }

    function act(action: () => void) {
        setOpen(false)
        action()
    }

    function copy() {
        void navigator.clipboard
            ?.writeText(message.content ?? '')
            .then(() => notify({ id: 'copied', icon: 'check', title: 'Текст скопирован' }))
    }

    const meta = (
        <span className={styles.meta}>
            {mine && !deleted && !message.pending && (
                <Icon
                    name={read ? 'check-double' : 'check'}
                    size={14}
                    color="var(--color-accent)"
                />
            )}
            {message.is_edited && !deleted && <span>изменено</span>}
            <time dateTime={message.created_at}>{formatTime(message.created_at)}</time>
        </span>
    )

    return (
        <div
            ref={rootRef}
            className={cn(
                styles.row,
                mine && styles.mine,
                continued && styles.continued,
                message.pending && styles.pending,
                appear && styles.appear,
            )}
            data-message-id={message.id}
        >
            {mine && meta}

            {avatar !== 'none' && (
                <span className={styles.avatar}>
                    {avatar === 'show' && <AuthorAvatar userId={message.sender_id} />}
                </span>
            )}

            <div className={styles.column}>
                <div
                    className={cn(
                        styles.bubble,
                        deleted && styles.deleted,
                        highlighted && styles.highlighted,
                    )}
                >
                    {/* имя автора - внутри пузыря, над цитатой и текстом */}
                    {showAuthor && (
                        <AuthorName userId={message.sender_id} className={styles.bubbleAuthor} />
                    )}
                    {message.reply_to_message_id && (
                        <button
                            type="button"
                            className={styles.quote}
                            aria-label="Перейти к сообщению, на которое ответили"
                            onClick={() => onJump(message.reply_to_message_id!)}
                        >
                            {replyTo ? (
                                <>
                                    <AuthorName
                                        userId={replyTo.sender_id}
                                        className={styles.quoteAuthor}
                                    />
                                    <span className={styles.quoteText}>
                                        {replyTo.is_deleted ? 'Сообщение удалено' : replyTo.content}
                                    </span>
                                </>
                            ) : (
                                <span className={styles.quoteText}>Ответ на сообщение</span>
                            )}
                        </button>
                    )}
                    <div
                        className={styles.body}
                        tabIndex={interactive ? 0 : undefined}
                        aria-haspopup={interactive ? 'menu' : undefined}
                        aria-expanded={interactive ? open : undefined}
                        onContextMenu={(event) => {
                            if (!interactive) {
                                return
                            }
                            event.preventDefault()
                            toggle()
                        }}
                    >
                        <span className={styles.text}>
                            {deleted ? 'Сообщение удалено' : message.content}
                        </span>
                    </div>
                </div>

                {open && (
                    <div
                        role="menu"
                        aria-label="Действия с сообщением"
                        className={cn(
                            styles.actions,
                            mine && styles.actionsMine,
                            above && styles.actionsAbove,
                        )}
                    >
                        <IconButton
                            role="menuitem"
                            icon="reply"
                            aria-label="Ответить"
                            onClick={() => act(() => onReply(message))}
                        />
                        <IconButton
                            role="menuitem"
                            icon="copy"
                            aria-label="Копировать"
                            onClick={() => act(copy)}
                        />
                        {canEdit && (
                            <IconButton
                                role="menuitem"
                                icon="edit"
                                aria-label="Изменить"
                                onClick={() => act(() => onEdit(message))}
                            />
                        )}
                        {canDelete && (
                            <IconButton
                                role="menuitem"
                                icon="trash"
                                aria-label="Удалить"
                                className={styles.danger}
                                onClick={() => act(() => onDelete(message))}
                            />
                        )}
                    </div>
                )}
            </div>

            {!mine && meta}
        </div>
    )
}
