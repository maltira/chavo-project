import { Fragment, useEffect, useLayoutEffect, useRef, useState } from 'react'

import type { ConversationSummary } from '@/entities/conversation'
import { isSystem } from '@/entities/message'
import type { Message, PinnedMessage } from '@/entities/message'
import { dayKey, formatDay, notify } from '@/shared/lib'
import { IconButton } from '@/shared/ui'

import styles from './MessageList.module.scss'
import { AppearRow } from './AppearRow'
import { MessageBubble } from './MessageBubble'
import { SystemMessage } from './SystemMessage'

interface Props {
    chat: ConversationSummary
    // свежие первыми, как отдаёт сервер
    messages: Message[]
    myId: string | undefined
    readBoundary: number | undefined
    hasOlder: boolean
    loadingOlder: boolean
    loadOlder: () => void
    // первое непрочитанное на момент открытия чата: над ним разделитель
    firstUnreadId: string | null
    // догружает старую историю, пока сообщение не найдено; false - его нет или оно скрыто
    findMessage: (id: string) => Promise<boolean>
    // самое свежее сообщение, которое пользователь увидел на экране
    onSeen: (id: string) => void
    onReply: (message: Message) => void
    onEdit: (message: Message) => void
    onDelete: (message: Message) => void
    pins: PinnedMessage[]
    onPin: (message: Message) => void
    // переход к сообщению снаружи (плашка закрепов); n отличает повторный переход к тому же сообщению
    jumpRequest: { id: string; n: number } | null
    onForward: (message: Message) => void
    // режим выбора для пересылки: id отмеченных; null - режим выключен
    selected: string[] | null
    onToggleSelect: (message: Message) => void
}

// автор для раскладки: в «Избранном» пересланное с автором стоит слева, как чужое, и серии
// собираются по первому автору
const authorKey = (m: Message, saved: boolean) =>
    saved && m.is_forwarded ? `forwarded:${m.forwarded_from_user_id ?? ''}` : m.sender_id

// одна серия - подряд идущие сообщения одного автора; системное серию прерывает
const sameAuthor = (a: Message | undefined, b: Message, saved: boolean) =>
    !!a && !isSystem(a) && !isSystem(b) && authorKey(a, saved) === authorKey(b, saved)

// насколько надо уйти от низа истории, чтобы появилась кнопка «вниз»
const SCROLL_DOWN_THRESHOLD = 300
// сколько держится подсветка сообщения после перехода по цитате
const HIGHLIGHT_MS = 1500

// история: column-reverse держит прокрутку у низа, новые сообщения сразу на виду,
// а подгрузка старых сверху не сдвигает то, что читает пользователь
export function MessageList({
    chat,
    messages,
    myId,
    readBoundary,
    hasOlder,
    loadingOlder,
    loadOlder,
    firstUnreadId,
    findMessage,
    onSeen,
    onReply,
    onEdit,
    onDelete,
    pins,
    onPin,
    jumpRequest,
    onForward,
    selected,
    onToggleSelect,
}: Props) {
    const sentinelRef = useRef<HTMLDivElement>(null)
    const scrollerRef = useRef<HTMLDivElement>(null)
    const dividerRef = useRef<HTMLParagraphElement>(null)
    const scrolledToUnreadRef = useRef(false)
    const [awayFromBottom, setAwayFromBottom] = useState(false)
    const [highlighted, setHighlighted] = useState<string | null>(null)
    const newest = messages[0]
    const newestRef = useRef(newest?.id)

    // своё новое сообщение (в том числе ещё не подтверждённое) - история уезжает в самый низ,
    // где бы пользователь ни читал; чужие сообщения прокрутку не трогают
    useEffect(() => {
        if (!newest || newest.id === newestRef.current) {
            return
        }
        newestRef.current = newest.id
        if (newest.sender_id === myId) {
            scrollerRef.current?.scrollTo?.({ top: 0, behavior: 'smooth' })
        }
    }, [newest, myId])
    // сообщения новее самого свежего на момент открытия пришли при открытом чате - они въезжают снизу;
    // история при открытии и подгрузка старых страниц появляются без анимации
    const [openedAt] = useState(() => newest?.created_at ?? '')
    const appears = (message: Message) => message.created_at > openedAt
    const isGroup = chat.conversation_type === 'group'
    const isAdmin = chat.my_role === 'admin'
    const saved = chat.is_saved
    const byId = new Map(messages.map((m) => [m.id, m]))

    // старые сообщения подгружаются, когда верх истории подошёл к видимой области
    useEffect(() => {
        const node = sentinelRef.current
        if (!node || !hasOlder || loadingOlder || typeof IntersectionObserver === 'undefined') {
            return
        }
        const observer = new IntersectionObserver(
            (entries) => {
                if (entries.some((entry) => entry.isIntersecting)) {
                    loadOlder()
                }
            },
            { rootMargin: '300px' },
        )
        observer.observe(node)
        return () => observer.disconnect()
    }, [hasOlder, loadingOlder, loadOlder, messages.length])

    // при открытии чата с непрочитанными история встаёт на разделитель, а не в самый низ;
    // до отрисовки, чтобы наблюдатель ниже не успел «увидеть» нижние сообщения
    useLayoutEffect(() => {
        if (firstUnreadId && !scrolledToUnreadRef.current && dividerRef.current) {
            scrolledToUnreadRef.current = true
            dividerRef.current.scrollIntoView?.({ block: 'center' })
        }
    }, [firstUnreadId])

    // что попало на экран, считается прочитанным: сообщение видно хотя бы наполовину
    // (или занимает половину высоты истории, если оно длиннее экрана)
    const seenRef = useRef(new Set<string>())
    const ids = messages.map((m) => m.id).join(',')
    useEffect(() => {
        const scroller = scrollerRef.current
        if (!scroller || typeof IntersectionObserver === 'undefined') {
            return
        }
        const observer = new IntersectionObserver(
            (entries) => {
                const rootHeight = scroller.clientHeight
                for (const entry of entries) {
                    const id = (entry.target as HTMLElement).dataset.messageId
                    const visible =
                        entry.intersectionRatio >= 0.5 ||
                        entry.intersectionRect.height >= rootHeight / 2
                    if (id && entry.isIntersecting && visible) {
                        seenRef.current.add(id)
                    }
                }
                // история идёт от свежих к старым: первое увиденное и есть самое свежее
                const freshest = messages.find((m) => seenRef.current.has(m.id))
                if (freshest) {
                    onSeen(freshest.id)
                }
            },
            { root: scroller, threshold: [0, 0.5, 1] },
        )
        scroller.querySelectorAll('[data-message-id]').forEach((row) => observer.observe(row))
        return () => observer.disconnect()
        // eslint-disable-next-line react-hooks/exhaustive-deps -- пересоздаём только при смене состава
    }, [ids, onSeen])

    // подсветка гаснет сама
    useEffect(() => {
        if (!highlighted) {
            return
        }
        const timer = setTimeout(() => setHighlighted(null), HIGHLIGHT_MS)
        return () => clearTimeout(timer)
    }, [highlighted])

    // в column-reverse низ истории - scrollTop 0, вверх значения отрицательные
    function onScroll() {
        const scroller = scrollerRef.current
        if (scroller) {
            setAwayFromBottom(Math.abs(scroller.scrollTop) > SCROLL_DOWN_THRESHOLD)
        }
    }

    function scrollToBottom() {
        scrollerRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
    }

    async function jumpTo(id: string) {
        if (!byId.has(id) && !(await findMessage(id))) {
            notify({ id: 'jump', icon: 'warn', title: 'Сообщение недоступно' })
            return
        }
        // после догрузки сообщение появляется в разметке на следующем кадре
        requestAnimationFrame(() => {
            scrollerRef.current
                ?.querySelector(`[data-message-id="${CSS.escape(id)}"]`)
                ?.scrollIntoView?.({ behavior: 'smooth', block: 'center' })
            setHighlighted(id)
        })
    }

    useEffect(() => {
        if (jumpRequest) {
            void jumpTo(jumpRequest.id)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps -- переход только по новому запросу
    }, [jumpRequest])

    // в группе закрепляют только админы; в личном чате - любой из двоих
    const canPin = !isGroup || isAdmin
    const pinnedIds = new Set(pins.map((p) => p.message.id))

    return (
        <div className={styles.root}>
            <div
                ref={scrollerRef}
                className={styles.scroller}
                role="log"
                aria-label="Сообщения"
                aria-live="polite"
                onScroll={onScroll}
            >
                {messages.map((message, index) => {
                    const older = messages[index + 1]
                    const own = message.sender_id === myId
                    // сторона пузыря: в «Избранном» пересланное с автором - слева
                    const mine = own && !(saved && message.is_forwarded)
                    const newDay = !older || dayKey(older.created_at) !== dayKey(message.created_at)
                    // имя автора - у первого сообщения из серии подряд идущих чужих в группе
                    const showAuthor =
                        isGroup && !mine && (newDay || !sameAuthor(older, message, saved))
                    // аватар - у нижнего (самого свежего) сообщения серии; у остальных - пустое место
                    // той же ширины, чтобы пузыри серии стояли ровно
                    const newer = messages[index - 1]
                    const lastInSeries =
                        !sameAuthor(newer, message, saved) ||
                        dayKey(newer!.created_at) !== dayKey(message.created_at)
                    // в группе - у чужих, в «Избранном» - у пересланных от другого автора
                    const avatar =
                        (isGroup || saved) && !mine ? (lastInSeries ? 'show' : 'space') : 'none'
                    // продолжение серии: сверху сообщение того же автора за тот же день без разделителя -
                    // между ними отступ меньше обычного
                    const continued =
                        !newDay && sameAuthor(older, message, saved) && message.id !== firstUnreadId

                    return (
                        <Fragment key={message.local_key ?? message.id}>
                            <AppearRow appear={appears(message)} continued={continued}>
                                {isSystem(message) ? (
                                    <SystemMessage
                                        message={message}
                                        appear={appears(message)}
                                        quoted={
                                            message.reply_to_message_id
                                                ? byId.get(message.reply_to_message_id)
                                                : undefined
                                        }
                                        onJump={(id) => void jumpTo(id)}
                                    />
                                ) : (
                                    <MessageBubble
                                        message={message}
                                        mine={mine}
                                        read={readBoundary !== undefined && index >= readBoundary}
                                        receipts={!chat.is_saved}
                                        showAuthor={showAuthor}
                                        avatar={avatar}
                                        avatarUserId={
                                            saved && message.is_forwarded
                                                ? (message.forwarded_from_user_id ?? null)
                                                : undefined
                                        }
                                        // у раскрывающейся строки отступ серии задаёт AppearRow
                                        continued={continued && !appears(message)}
                                        tail={lastInSeries}
                                        replyTo={
                                            message.reply_to_message_id
                                                ? byId.get(message.reply_to_message_id)
                                                : undefined
                                        }
                                        // пересланное - чужой текст: не правится
                                        canEdit={own && !message.is_forwarded}
                                        canDelete={own || (isGroup && isAdmin)}
                                        pinned={canPin ? pinnedIds.has(message.id) : null}
                                        onPin={onPin}
                                        onForward={onForward}
                                        selecting={selected !== null}
                                        selected={!!selected?.includes(message.id)}
                                        onToggleSelect={onToggleSelect}
                                        highlighted={highlighted === message.id}
                                        appear={appears(message)}
                                        onReply={onReply}
                                        onEdit={onEdit}
                                        onDelete={onDelete}
                                        onJump={(id) => void jumpTo(id)}
                                    />
                                )}
                            </AppearRow>
                            {message.id === firstUnreadId && (
                                <p ref={dividerRef} className={styles.unread}>
                                    Непрочитанные сообщения
                                </p>
                            )}
                            {/* в column-reverse разделитель после сообщения стоит над ним */}
                            {newDay && (
                                <p className={styles.day}>{formatDay(message.created_at)}</p>
                            )}
                        </Fragment>
                    )
                })}
                {loadingOlder && <p className={styles.day}>Загружаем…</p>}
                {hasOlder && <div ref={sentinelRef} className={styles.sentinel} />}
            </div>

            {awayFromBottom && (
                <IconButton
                    icon="arrow-right"
                    aria-label="Вниз"
                    className={styles.down}
                    onClick={scrollToBottom}
                />
            )}
        </div>
    )
}
