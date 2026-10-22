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
    // галочки прочтения у своих сообщений; в «Избранном» читать некому
    receipts: boolean
    showAuthor: boolean // имя автора над пузырём
    // аватар автора слева (чужие сообщения в группе, пересланные в «Избранном»): show - у нижнего
    // в серии, space - место под него
    avatar: 'show' | 'space' | 'none'
    // чей аватар: по умолчанию отправителя; у пересланного в «Избранном» - первого автора (null - скрыт)
    avatarUserId?: string | null
    // продолжение серии того же автора: ближе к сообщению над ним
    continued: boolean
    // последнее в серии: хвостик пузыря в сторону автора
    tail: boolean
    replyTo: Message | null | undefined
    canEdit: boolean
    canDelete: boolean
    // закреп: null - закреплять нельзя (не админ группы), иначе закреплено ли сообщение
    pinned: boolean | null
    highlighted: boolean // подсветка после перехода по цитате
    appear: boolean // пришло при открытом чате: въезжает снизу
    onReply: (message: Message) => void
    onEdit: (message: Message) => void
    onDelete: (message: Message) => void
    onPin: (message: Message) => void
    onForward: (message: Message) => void
    onJump: (messageId: string) => void
    // режим выбора для пересылки: нажатие отмечает сообщение, меню действий не открывается
    selecting: boolean
    selected: boolean
    onToggleSelect: (message: Message) => void
}

// аватар автора; нажатие открывает его профиль; null - автор скрыл имя: заглушка без ссылки
function AuthorAvatar({ userId }: { userId: string | null }) {
    const user = useUser(userId)
    const openProfile = useProfileCard((state) => state.open)
    const profile = user.data
    if (!userId) {
        return <UserAvatar size={32} />
    }
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

// «Переслано от <имя>» над текстом; имя - ссылка на профиль; автор мог скрыть имя
function ForwardedFrom({ userId }: { userId: string | null }) {
    const user = useUser(userId)
    const openProfile = useProfileCard((state) => state.open)
    if (!userId) {
        return <span className={styles.forwarded}>Переслано от скрытого автора</span>
    }
    return (
        <span className={styles.forwarded}>
            Переслано от{' '}
            <button
                type="button"
                className={styles.forwardedName}
                onClick={() => openProfile(userId)}
            >
                {user.data?.display_name ?? '…'}
            </button>
        </span>
    )
}

// пузырь сообщения
export function MessageBubble({
    message,
    mine,
    read,
    receipts,
    showAuthor,
    avatar,
    avatarUserId,
    continued,
    tail,
    replyTo,
    canEdit,
    canDelete,
    pinned,
    highlighted,
    appear,
    onReply,
    onEdit,
    onDelete,
    onPin,
    onForward,
    onJump,
    selecting,
    selected,
    onToggleSelect,
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

    // время, «изменено» и галочки - в правом нижнем углу пузыря, как в Telegram
    const metaContent = (
        <>
            {message.is_edited && !deleted && <span>изменено</span>}
            <time dateTime={message.created_at}>{formatTime(message.created_at)}</time>
            {mine && receipts && !deleted && !message.pending && (
                <Icon name={read ? 'check-double' : 'check'} size={14} color="currentColor" />
            )}
        </>
    )

    // в режиме выбора вся строка - флажок; удалённые и неотправленные не пересылаются
    const selectable = selecting && interactive
    const toggleSelect = () => selectable && onToggleSelect(message)

    return (
        <div
            ref={rootRef}
            className={cn(
                styles.row,
                mine && styles.mine,
                continued && styles.continued,
                message.pending && styles.pending,
                appear && styles.appear,
                selecting && styles.selecting,
                selected && styles.selected,
            )}
            data-message-id={message.id}
            role={selectable ? 'checkbox' : undefined}
            aria-checked={selectable ? selected : undefined}
            aria-label={selectable ? `Выбрать сообщение: ${message.content ?? ''}` : undefined}
            tabIndex={selectable ? 0 : undefined}
            onClick={selecting ? toggleSelect : undefined}
            onKeyDown={
                selectable
                    ? (event) => {
                          if (event.key === ' ' || event.key === 'Enter') {
                              event.preventDefault()
                              toggleSelect()
                          }
                      }
                    : undefined
            }
        >
            {selecting && (
                <span className={cn(styles.selectMark, !interactive && styles.selectMarkOff)}>
                    {selected && <Icon name="check" size={14} color="currentColor" />}
                </span>
            )}

            {avatar !== 'none' && (
                <span className={styles.avatar}>
                    {avatar === 'show' && (
                        <AuthorAvatar
                            userId={avatarUserId === undefined ? message.sender_id : avatarUserId}
                        />
                    )}
                </span>
            )}

            <div className={styles.column}>
                <div
                    className={cn(
                        styles.bubble,
                        tail && styles.tail,
                        deleted && styles.deleted,
                        highlighted && styles.highlighted,
                    )}
                >
                    {/* имя автора - внутри пузыря, над цитатой и текстом */}
                    {showAuthor && (
                        <AuthorName userId={message.sender_id} className={styles.bubbleAuthor} />
                    )}
                    {message.is_forwarded && (
                        <ForwardedFrom userId={message.forwarded_from_user_id ?? null} />
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
                        tabIndex={interactive && !selecting ? 0 : undefined}
                        aria-haspopup={interactive && !selecting ? 'menu' : undefined}
                        aria-expanded={interactive && !selecting ? open : undefined}
                        onContextMenu={(event) => {
                            if (!interactive || selecting) {
                                return
                            }
                            event.preventDefault()
                            toggle()
                        }}
                    >
                        <span className={styles.text}>
                            {deleted ? 'Сообщение удалено' : message.content}
                            {/* невидимая копия занимает место в конце последней строки:
                                если не помещается, время уходит на новую строку */}
                            <span className={styles.metaSpace} aria-hidden="true">
                                {metaContent}
                            </span>
                        </span>
                        <span className={styles.meta}>{metaContent}</span>
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
                            icon="forward"
                            aria-label="Переслать"
                            onClick={() => act(() => onForward(message))}
                        />
                        <IconButton
                            role="menuitem"
                            icon="copy"
                            aria-label="Копировать"
                            onClick={() => act(copy)}
                        />
                        {pinned !== null && (
                            <IconButton
                                role="menuitem"
                                icon="pinned"
                                aria-label={pinned ? 'Открепить' : 'Закрепить'}
                                onClick={() => act(() => onPin(message))}
                            />
                        )}
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
        </div>
    )
}
