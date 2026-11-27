import { useEffect, useRef, useState } from 'react'

import type { Message } from '@/entities/message'
import { useFavoriteEmojis, useProfileCard, UserAvatar, useUser } from '@/entities/user'
import { cn, copyRich, formatTime, notify, useDismiss } from '@/shared/lib'
import { EmojiPicker, Icon, IconButton, RichText } from '@/shared/ui'

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
    // кто прочитал сообщение (свои прочитанные сообщения в группе)
    onShowReaders?: (message: Message) => void
    onToggleReaction?: (message: Message, emoji: string, active: boolean) => void
    onShowReactions?: (message: Message, initialEmoji?: string) => void
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

// высота панели действий с полосой быстрых реакций и отступом от сообщения
const ACTIONS_SPACE = 124
const QUICK_REACTIONS = ['👍', '❤️', '😂', '😮', '😢', '🔥']
// задержка долгого нажатия на тач-экране, мс
const LONG_PRESS_MS = 450
const LONG_PRESS_MOVE = 10

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
    onShowReaders,
    onToggleReaction,
    onShowReactions,
    onJump,
    selecting,
    selected,
    onToggleSelect,
}: Props) {
    const [open, setOpen] = useState(false)
    const [pickerOpen, setPickerOpen] = useState(false)
    const favorites = useFavoriteEmojis(pickerOpen)
    const openProfile = useProfileCard((state) => state.open)

    const [above, setAbove] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)
    const bodyRef = useRef<HTMLDivElement>(null)
    const longPressRef = useRef<{ timer: ReturnType<typeof setTimeout>; x: number; y: number } | null>(
        null,
    )

    function clearLongPress() {
        if (longPressRef.current) {
            clearTimeout(longPressRef.current.timer)
            longPressRef.current = null
        }
    }

    useEffect(() => clearLongPress, [])

    useDismiss(open, rootRef, () => {
        const inside = rootRef.current?.contains(document.activeElement)
        setOpen(false)
        setPickerOpen(false)
        if (inside) {
            bodyRef.current?.focus()
        }
    })

    const deleted = message.is_deleted
    const interactive = !deleted && !message.pending
    const reactions = message.reactions ?? []

    function computeAbove() {
        const row = rootRef.current
        const log = row?.closest('[role="log"]')
        if (row && log) {
            const space = log.getBoundingClientRect().bottom - row.getBoundingClientRect().bottom
            setAbove(space < ACTIONS_SPACE)
        }
    }

    function openMenu() {
        if (!open) {
            computeAbove()
        }
        setPickerOpen(false)
        setOpen(true)
    }

    function toggle() {
        if (!open) {
            computeAbove()
            setPickerOpen(false)
        } else {
            setPickerOpen(false)
        }
        setOpen((value) => !value)
    }

    function act(action: () => void) {
        setOpen(false)
        setPickerOpen(false)
        action()
    }

    function pickReaction(emoji: string) {
        if (!onToggleReaction) {
            return
        }
        const existing = reactions.find((r) => r.emoji === emoji)
        act(() => onToggleReaction(message, emoji, !existing?.me))
    }

    function copy() {
        void copyRich({
            text: message.content ?? '',
            entities: message.entities ?? [],
        }).then(() => notify({ id: 'copied', icon: 'check', title: 'Текст скопирован' }))
    }

    // время, «изменено», «без звука» и галочки - в правом нижнем углу пузыря, как в Telegram
    const metaContent = (
        <>
            {message.is_silent && !deleted && (
                <span
                    role="img"
                    className={styles.silentIcon}
                    title="Отправлено без звука"
                    aria-label="Отправлено без звука"
                >
                    <Icon name="notif-off" size={12} color="currentColor" />
                </span>
            )}
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

            {mine && interactive && !selecting && (
                <IconButton
                    icon="menu-dot"
                    size={28}
                    aria-label="Действия с сообщением"
                    aria-haspopup="menu"
                    aria-expanded={open}
                    tabIndex={-1}
                    className={cn(styles.menuTrigger, open && styles.menuTriggerOpen)}
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={toggle}
                />
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
                                        {replyTo.is_deleted ? (
                                            'Сообщение удалено'
                                        ) : (
                                            <RichText
                                                text={replyTo.content ?? ''}
                                                entities={replyTo.entities}
                                                interactive={false}
                                            />
                                        )}
                                    </span>
                                </>
                            ) : (
                                <span className={styles.quoteText}>Ответ на сообщение</span>
                            )}
                        </button>
                    )}
                    <div
                        ref={bodyRef}
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
                        onKeyDown={(event) => {
                            if (!interactive || selecting) {
                                return
                            }
                            if (
                                event.key === 'Enter' ||
                                event.key === ' ' ||
                                event.key === 'ContextMenu' ||
                                (event.key === 'F10' && event.shiftKey)
                            ) {
                                event.preventDefault()
                                toggle()
                            }
                        }}
                        onPointerDown={(event) => {
                            if (!interactive || selecting || event.pointerType !== 'touch') {
                                return
                            }
                            clearLongPress()
                            const x = event.clientX
                            const y = event.clientY
                            longPressRef.current = {
                                x,
                                y,
                                timer: setTimeout(() => {
                                    longPressRef.current = null
                                    openMenu()
                                }, LONG_PRESS_MS),
                            }
                        }}
                        onPointerMove={(event) => {
                            const lp = longPressRef.current
                            if (!lp) {
                                return
                            }
                            if (
                                Math.hypot(event.clientX - lp.x, event.clientY - lp.y) >
                                LONG_PRESS_MOVE
                            ) {
                                clearLongPress()
                            }
                        }}
                        onPointerUp={clearLongPress}
                        onPointerCancel={clearLongPress}
                    >
                        <span className={styles.text}>
                            {deleted ? (
                                'Сообщение удалено'
                            ) : (
                                <RichText
                                    text={message.content ?? ''}
                                    entities={message.entities}
                                    onMentionClick={openProfile}
                                />
                            )}
                            {/* невидимая копия занимает место в конце последней строки:
                                если не помещается, время уходит на новую строку */}
                            <span className={styles.metaSpace} aria-hidden="true">
                                {metaContent}
                            </span>
                        </span>
                        <span className={styles.meta}>{metaContent}</span>
                    </div>
                    {!deleted && message.link_preview && (
                        <a
                            href={message.link_preview.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={styles.linkPreview}
                            onClick={(event) => event.stopPropagation()}
                        >
                            <div className={styles.linkPreviewBody}>
                                {message.link_preview.site_name && (
                                    <span className={styles.linkPreviewSite}>
                                        {message.link_preview.site_name}
                                    </span>
                                )}
                                {message.link_preview.title && (
                                    <span className={styles.linkPreviewTitle}>
                                        {message.link_preview.title}
                                    </span>
                                )}
                                {message.link_preview.description && (
                                    <span className={styles.linkPreviewDesc}>
                                        {message.link_preview.description}
                                    </span>
                                )}
                            </div>
                            {message.link_preview.image_url && (
                                <img
                                    src={message.link_preview.image_url}
                                    alt=""
                                    className={styles.linkPreviewImg}
                                    loading="lazy"
                                />
                            )}
                        </a>
                    )}
                </div>

                {!deleted && reactions.length > 0 && (
                    <div className={styles.reactions} aria-label="Реакции">
                        {reactions.map((r) => (
                            <button
                                key={r.emoji}
                                type="button"
                                disabled={!interactive || selecting || !onToggleReaction}
                                aria-pressed={r.me}
                                aria-label={`Реакция ${r.emoji}: ${r.count}`}
                                title="Нажмите, чтобы поставить или снять; ПКМ — кто отреагировал"
                                className={cn(styles.reactionPill, r.me && styles.reactionMine)}
                                onClick={() => onToggleReaction?.(message, r.emoji, !r.me)}
                                onContextMenu={(event) => {
                                    if (!onShowReactions) {
                                        return
                                    }
                                    event.preventDefault()
                                    onShowReactions(message, r.emoji)
                                }}
                            >
                                <span className={styles.reactionEmoji}>{r.emoji}</span>
                                <span className={styles.reactionCount}>{r.count}</span>
                            </button>
                        ))}
                    </div>
                )}

                {open && (
                    <div
                        role="menu"
                        aria-label="Действия с сообщением"
                        className={cn(
                            styles.actionsWrap,
                            mine && styles.actionsMine,
                            above && styles.actionsAbove,
                        )}
                    >
                        {onToggleReaction && (
                            <div className={styles.quickReactions} aria-label="Быстрые реакции">
                                {QUICK_REACTIONS.map((emoji) => {
                                    const active = reactions.some((r) => r.emoji === emoji && r.me)
                                    return (
                                        <button
                                            key={emoji}
                                            type="button"
                                            className={cn(
                                                styles.quickEmoji,
                                                active && styles.quickEmojiActive,
                                            )}
                                            aria-label={`Реакция ${emoji}`}
                                            aria-pressed={active}
                                            onClick={() => pickReaction(emoji)}
                                        >
                                            {emoji}
                                        </button>
                                    )
                                })}
                                <IconButton
                                    icon="add"
                                    size={28}
                                    aria-label="Выбрать реакцию"
                                    aria-expanded={pickerOpen}
                                    isActive={pickerOpen}
                                    onClick={() => setPickerOpen((v) => !v)}
                                />
                            </div>
                        )}
                        {pickerOpen && onToggleReaction && (
                            <div className={styles.reactionPicker}>
                                <EmojiPicker
                                    onPick={(emoji) => pickReaction(emoji)}
                                    favorites={favorites.list}
                                    onToggleFavorite={favorites.toggle}
                                />
                            </div>
                        )}
                        <div className={styles.actions}>
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
                            {onShowReactions && reactions.length > 0 && (
                                <IconButton
                                    role="menuitem"
                                    icon="heart"
                                    aria-label="Кто отреагировал"
                                    onClick={() => act(() => onShowReactions(message))}
                                />
                            )}
                            {onShowReaders && (
                                <IconButton
                                    role="menuitem"
                                    icon="check-double"
                                    aria-label="Кто прочитал"
                                    onClick={() => act(() => onShowReaders(message))}
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
                    </div>
                )}
            </div>

            {!mine && interactive && !selecting && (
                <IconButton
                    icon="menu-dot"
                    size={28}
                    aria-label="Действия с сообщением"
                    aria-haspopup="menu"
                    aria-expanded={open}
                    tabIndex={-1}
                    className={cn(styles.menuTrigger, open && styles.menuTriggerOpen)}
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={toggle}
                />
            )}
        </div>
    )
}
