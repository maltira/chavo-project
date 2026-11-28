import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

import {
    MAX_DISTINCT_REACTIONS_PER_MSG,
    MAX_USER_REACTIONS_PER_MSG,
} from '@/entities/message'
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

// отступ всплывающего меню от краёв окна
const MENU_EDGE = 8
const QUICK_REACTIONS = ['👍', '❤️', '😂', '😮', '😢', '🔥']
// задержка долгого нажатия на тач-экране, мс
const LONG_PRESS_MS = 450
const LONG_PRESS_MOVE = 10

interface MenuAnchor {
    x: number
    y: number
    align: 'start' | 'end'
}

// одновременно может быть открыто только одно меню действий с сообщением
let closeActiveMenu: (() => void) | null = null

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
    const [menuAnchor, setMenuAnchor] = useState<MenuAnchor | null>(null)
    const [placedMenu, setPlacedMenu] = useState<{ x: number; y: number } | null>(null)
    const [pickerOpen, setPickerOpen] = useState(false)
    const favorites = useFavoriteEmojis(pickerOpen)
    const openProfile = useProfileCard((state) => state.open)

    const rootRef = useRef<HTMLDivElement>(null)
    const bodyRef = useRef<HTMLDivElement>(null)
    const menuRef = useRef<HTMLDivElement>(null)
    const longPressRef = useRef<{ timer: ReturnType<typeof setTimeout>; x: number; y: number } | null>(
        null,
    )

    const open = menuAnchor !== null

    const closeSelf = useCallback(() => {
        setMenuAnchor(null)
        setPickerOpen(false)
    }, [])

    function closeMenu() {
        if (closeActiveMenu === closeSelf) {
            closeActiveMenu = null
        }
        closeSelf()
    }

    function clearLongPress() {
        if (longPressRef.current) {
            clearTimeout(longPressRef.current.timer)
            longPressRef.current = null
        }
    }

    useEffect(
        () => () => {
            clearLongPress()
            if (closeActiveMenu === closeSelf) {
                closeActiveMenu = null
            }
        },
        [closeSelf],
    )

    useDismiss(open, menuRef, () => {
        const activeEl = document.activeElement
        const inside =
            menuRef.current?.contains(activeEl) || rootRef.current?.contains(activeEl)
        closeMenu()
        if (inside) {
            bodyRef.current?.focus()
        }
    })

    // подгоняем позицию меню в пределы экрана после замера реальных размеров
    useLayoutEffect(() => {
        const node = menuRef.current
        if (!menuAnchor || !node) {
            setPlacedMenu(null)
            return
        }
        const width = node.offsetWidth
        const height = node.offsetHeight
        const rawLeft = menuAnchor.align === 'end' ? menuAnchor.x - width : menuAnchor.x
        const x = Math.max(MENU_EDGE, Math.min(rawLeft, window.innerWidth - width - MENU_EDGE))
        const y = Math.max(MENU_EDGE, Math.min(menuAnchor.y, window.innerHeight - height - MENU_EDGE))
        setPlacedMenu({ x, y })
    }, [menuAnchor, pickerOpen])

    const deleted = message.is_deleted
    const interactive = !deleted && !message.pending
    const reactions = message.reactions ?? []
    const hasReactions = !deleted && reactions.length > 0
    const myReactionsCount = reactions.reduce((sum, r) => sum + (r.me ? 1 : 0), 0)
    // когда достигнут лимит уникальных или своих реакций на сообщение - скрываем селектор эмодзи
    const canAddReaction =
        reactions.length < MAX_DISTINCT_REACTIONS_PER_MSG &&
        myReactionsCount < MAX_USER_REACTIONS_PER_MSG

    function openMenuAt(x: number, y: number) {
        if (closeActiveMenu && closeActiveMenu !== closeSelf) {
            closeActiveMenu()
        }
        closeActiveMenu = closeSelf
        setPickerOpen(false)
        setMenuAnchor({ x, y, align: mine ? 'end' : 'start' })
    }

    function openMenuNearBubble() {
        const rect = bodyRef.current?.getBoundingClientRect()
        if (rect) {
            openMenuAt(mine ? rect.right : rect.left, rect.bottom + 6)
        } else {
            openMenuAt(MENU_EDGE, MENU_EDGE)
        }
    }

    function toggle() {
        if (open) {
            closeMenu()
            return
        }
        openMenuNearBubble()
    }

    function act(action: () => void) {
        closeMenu()
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
                    onMouseDown={(event) => {
                        if (open) {
                            event.stopPropagation()
                        }
                    }}
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
                    onContextMenu={(event) => {
                        if (!interactive || selecting) {
                            return
                        }
                        event.preventDefault()
                        openMenuAt(event.clientX, event.clientY)
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
                                openMenuAt(x, y)
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
                        className={cn(styles.body, hasReactions && styles.bodyWithReactions)}
                        tabIndex={interactive && !selecting ? 0 : undefined}
                        aria-haspopup={interactive && !selecting ? 'menu' : undefined}
                        aria-expanded={interactive && !selecting ? open : undefined}
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
                            {!hasReactions && (
                                /* невидимая копия занимает место в конце последней строки:
                                   если не помещается, время уходит на новую строку */
                                <span className={styles.metaSpace} aria-hidden="true">
                                    {metaContent}
                                </span>
                            )}
                        </span>
                        {!hasReactions && <span className={styles.meta}>{metaContent}</span>}
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
                    {hasReactions && (
                        <div className={styles.reactionsFooter}>
                            <div className={styles.reactions} aria-label="Реакции">
                                {reactions.map((r) => (
                                    <button
                                        key={r.emoji}
                                        type="button"
                                        disabled={!interactive || selecting || !onToggleReaction}
                                        aria-pressed={r.me}
                                        aria-label={`Реакция ${r.emoji}: ${r.count}`}
                                        title="Нажмите, чтобы поставить или снять; ПКМ — кто отреагировал"
                                        className={cn(
                                            styles.reactionPill,
                                            r.me && styles.reactionMine,
                                        )}
                                        onClick={() => {
                                            if (
                                                !r.me &&
                                                myReactionsCount >= MAX_USER_REACTIONS_PER_MSG
                                            ) {
                                                return
                                            }
                                            onToggleReaction?.(message, r.emoji, !r.me)
                                        }}
                                        onContextMenu={(event) => {
                                            if (!onShowReactions) {
                                                return
                                            }
                                            event.preventDefault()
                                            event.stopPropagation()
                                            onShowReactions(message, r.emoji)
                                        }}
                                    >
                                        <span className={styles.reactionEmoji}>{r.emoji}</span>
                                        <span className={styles.reactionCount}>{r.count}</span>
                                    </button>
                                ))}
                            </div>
                            <span className={styles.metaInline}>{metaContent}</span>
                        </div>
                    )}
                </div>

                {open &&
                    createPortal(
                        <div
                            ref={menuRef}
                            role="menu"
                            aria-label="Действия с сообщением"
                            className={cn(styles.actionsWrap, mine && styles.actionsMine)}
                            style={{
                                left: (placedMenu ?? menuAnchor).x,
                                top: (placedMenu ?? menuAnchor).y,
                            }}
                            onContextMenu={(event) => event.preventDefault()}
                        >
                            {onToggleReaction && canAddReaction && (
                                <div className={styles.quickReactions} aria-label="Быстрые реакции">
                                    {QUICK_REACTIONS.map((emoji) => {
                                        const active = reactions.some(
                                            (r) => r.emoji === emoji && r.me,
                                        )
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
                            {pickerOpen && onToggleReaction && canAddReaction ? (
                                <div className={styles.reactionPicker}>
                                    <EmojiPicker
                                        onPick={(emoji) => pickReaction(emoji)}
                                        favorites={favorites.list}
                                        onToggleFavorite={favorites.toggle}
                                    />
                                </div>
                            ) : (
                                <div className={styles.actions}>
                                    <button
                                        type="button"
                                        role="menuitem"
                                        className={styles.actionItem}
                                        onClick={() => act(() => onReply(message))}
                                    >
                                        <Icon name="reply" size={18} color="currentColor" />
                                        <span>Ответить</span>
                                    </button>
                                    <button
                                        type="button"
                                        role="menuitem"
                                        className={styles.actionItem}
                                        onClick={() => act(() => onForward(message))}
                                    >
                                        <Icon name="forward" size={18} color="currentColor" />
                                        <span>Переслать</span>
                                    </button>
                                    <button
                                        type="button"
                                        role="menuitem"
                                        className={styles.actionItem}
                                        onClick={() => act(copy)}
                                    >
                                        <Icon name="copy" size={18} color="currentColor" />
                                        <span>Копировать</span>
                                    </button>
                                    {pinned !== null && (
                                        <button
                                            type="button"
                                            role="menuitem"
                                            className={styles.actionItem}
                                            onClick={() => act(() => onPin(message))}
                                        >
                                            <Icon name="pinned" size={18} color="currentColor" />
                                            <span>{pinned ? 'Открепить' : 'Закрепить'}</span>
                                        </button>
                                    )}
                                    {onShowReactions && reactions.length > 0 && (
                                        <button
                                            type="button"
                                            role="menuitem"
                                            className={styles.actionItem}
                                            onClick={() => act(() => onShowReactions(message))}
                                        >
                                            <Icon name="heart" size={18} color="currentColor" />
                                            <span>Реакции</span>
                                        </button>
                                    )}
                                    {onShowReaders && (
                                        <button
                                            type="button"
                                            role="menuitem"
                                            className={styles.actionItem}
                                            onClick={() => act(() => onShowReaders(message))}
                                        >
                                            <Icon name="check-double" size={18} color="currentColor" />
                                            <span>Кто прочитал</span>
                                        </button>
                                    )}
                                    {canEdit && (
                                        <button
                                            type="button"
                                            role="menuitem"
                                            className={styles.actionItem}
                                            onClick={() => act(() => onEdit(message))}
                                        >
                                            <Icon name="edit" size={18} color="currentColor" />
                                            <span>Изменить</span>
                                        </button>
                                    )}
                                    {canDelete && (
                                        <>
                                            <div className={styles.actionDivider} role="separator" />
                                            <button
                                                type="button"
                                                role="menuitem"
                                                className={cn(
                                                    styles.actionItem,
                                                    styles.actionDanger,
                                                )}
                                                onClick={() => act(() => onDelete(message))}
                                            >
                                                <Icon name="trash" size={18} color="currentColor" />
                                                <span>Удалить</span>
                                            </button>
                                        </>
                                    )}
                                </div>
                            )}
                        </div>,
                        document.body,
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
                    onMouseDown={(event) => {
                        if (open) {
                            event.stopPropagation()
                        }
                    }}
                    onClick={toggle}
                />
            )}
        </div>
    )
}
