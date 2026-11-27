import { useCallback, useEffect, useState } from 'react'

import { useConversation, useMembers } from '@/entities/conversation'
import {
    getDraft,
    MAX_FORWARD_MESSAGES,
    useMessages,
    usePins,
    useToggleReaction,
} from '@/entities/message'
import type { Message, PinnedMessage } from '@/entities/message'
import { useMe, useUser } from '@/entities/user'
import { MessageComposer } from '@/features/compose-message'
import type { ReplyTarget } from '@/features/compose-message'
import { ChatActionsMenu, useOpenChatInfo } from '@/features/chat-actions'
import { useDeleteMessage, useDeleteMessages } from '@/features/delete-message'
import { useMarkRead } from '@/features/mark-read'
import { ForwardModal } from '@/features/forward-message'
import { PinConfirmModal, usePinMessage } from '@/features/pin-message'
import { copyRich, notify, plural } from '@/shared/lib'
import type { MessageEntity } from '@/shared/lib'
import { Button, ConfirmModal, Icon, IconButton } from '@/shared/ui'
import type { MenuPosition } from '@/shared/ui'

import { firstUnreadId, readBoundary } from '../lib/readState'
import { BlockedNotice, useCanWrite } from './BlockedNotice'
import { ChatHeader } from './ChatHeader'
import { HeaderSkeleton, MessagesSkeleton } from './ChatSkeleton'
import styles from './ChatWindow.module.scss'
import { MessageList } from './MessageList'
import { PinnedBar } from './PinnedBar'
import { ReactorsModal } from './ReactorsModal'
import { ReadersModal } from './ReadersModal'
import { SelectionBar } from './SelectionBar'

interface Props {
    conversationId: string
}

// окно открытого чата
export function ChatWindow({ conversationId }: Props) {
    const me = useMe()
    const myId = me.data?.user_id
    const chat = useConversation(conversationId)
    // в личном чате писать нельзя, если кто-то из двоих заблокировал другого
    const peerId = chat.data?.conversation_type === 'direct' ? chat.data.peer_id : null
    const canWrite = useCanWrite(peerId)
    const history = useMessages(conversationId)
    const members = useMembers(conversationId)
    const remove = useDeleteMessage()
    const removeBatch = useDeleteMessages()
    const openInfo = useOpenChatInfo()
    const pins = usePins(conversationId)
    const pin = usePinMessage(conversationId)
    const toggleReaction = useToggleReaction(conversationId, myId)
    // подтверждение закрепа или открепления в личном чате
    const [pinDialog, setPinDialog] = useState<{ mode: 'pin' | 'unpin'; messageId: string } | null>(
        null,
    )
    const [jumpRequest, setJumpRequest] = useState<{ id: string; n: number } | null>(null)
    // режим выбора для пересылки, копирования и удаления: отмеченные сообщения; null - режим выключен
    const [selected, setSelected] = useState<string[] | null>(null)
    const [forwarding, setForwarding] = useState(false)
    const [confirmingDelete, setConfirmingDelete] = useState(false)
    // просмотр списка прочитавших сообщение в группе
    const [readersOf, setReadersOf] = useState<string | null>(null)
    // просмотр списка отреагировавших на сообщение
    const [reactorsOf, setReactorsOf] = useState<{
        messageId: string
        initialEmoji?: string
    } | null>(null)

    // Esc выходит из режима выбора (открытые окна пересылки/удаления Esc закрывают сами)
    useEffect(() => {
        if (selected === null || forwarding || confirmingDelete) {
            return
        }
        const onKey = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                setSelected(null)
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [selected, forwarding, confirmingDelete])

    function toggleSelect(message: Message) {
        setSelected((ids) => {
            const current = ids ?? []
            if (current.includes(message.id)) {
                return current.filter((id) => id !== message.id)
            }
            if (current.length >= MAX_FORWARD_MESSAGES) {
                notify({
                    id: 'forward-limit',
                    icon: 'warn',
                    title: `Можно выбрать не больше ${MAX_FORWARD_MESSAGES} сообщений`,
                })
                return current
            }
            return [...current, message.id]
        })
    }

    // меню «⋮» в шапке - под кнопкой
    const [menuAt, setMenuAt] = useState<MenuPosition | null>(null)
    const [reply, setReply] = useState<Message | null>(null)
    // ответ, восстановленный из черновика чата
    const [draftReply, setDraftReply] = useState<ReplyTarget | null>(
        () => getDraft(conversationId)?.replyTo ?? null,
    )
    const [editing, setEditing] = useState<Message | null>(null)
    const replyAuthor = useUser(reply && reply.sender_id !== myId ? reply.sender_id : null)

    const messages = history.data ?? []

    // число непрочитанных запоминается при открытии: прочтение сразу обнулит его в списке
    const [unreadAtOpen] = useState(() => chat.data?.unread_count ?? 0)
    // разделитель ставится один раз, когда история загрузилась, и потом не двигается
    const [divider, setDivider] = useState<{ id: string | null } | null>(null)
    if (divider === null && messages.length > 0) {
        setDivider({ id: firstUnreadId(messages, unreadAtOpen, myId) })
    }
    // самое свежее сообщение, которое побывало на экране; до него курсор прочтения
    const [seenId, setSeenId] = useState<string>()
    useMarkRead(conversationId, seenId)

    const { fetchNextPage, fetchPreviousPage, loadAround, resetToLatest } = history
    const loadOlder = useCallback(
        () => void fetchNextPage({ cancelRefetch: false }),
        [fetchNextPage],
    )
    const loadNewer = useCallback(
        () => void fetchPreviousPage({ cancelRefetch: false }),
        [fetchPreviousPage],
    )
    const returnToLatest = useCallback(() => void resetToLatest(), [resetToLatest])

    // переход по цитате или закрепу к далёкому сообщению: загружаем окно around одним запросом
    const findMessage = useCallback((id: string) => loadAround(id), [loadAround])

    if (chat.isError) {
        return (
            <div className={styles.state}>
                <Icon name="shape-warn" size={67} color="var(--color-text)" />
                <p className={styles.stateText}>Чат недоступен</p>
            </div>
        )
    }

    if (!chat.data) {
        return (
            <div className={styles.root}>
                <HeaderSkeleton />
                <MessagesSkeleton />
            </div>
        )
    }

    const isGroup = chat.data.conversation_type === 'group'
    const isAdmin = chat.data.my_role === 'admin'
    const pinList = pins.data ?? []

    // выбранные сообщения в хронологическом порядке (старые сверху)
    const selectedSet = selected ? new Set(selected) : null
    const selectedMessages = selectedSet
        ? messages.filter((m) => selectedSet.has(m.id)).reverse()
        : []
    const canDeleteSelected =
        selectedMessages.length > 0 &&
        selectedMessages.every((m) => m.sender_id === myId || (isGroup && isAdmin))

    function copySelected() {
        if (selectedMessages.length === 0) {
            return
        }
        let text = ''
        const entities: MessageEntity[] = []
        selectedMessages.forEach((m, index) => {
            if (index > 0) {
                text += '\n\n'
            }
            const baseOffset = text.length
            const part = m.content ?? ''
            text += part
            for (const e of m.entities ?? []) {
                entities.push({ type: e.type, offset: baseOffset + e.offset, length: e.length })
            }
        })
        void copyRich({ text, entities }).then(() => {
            notify({ id: 'copied', icon: 'check', title: 'Скопировано' })
            setSelected(null)
        })
    }

    function confirmDeleteSelected() {
        if (!selected || selected.length === 0) {
            return
        }
        removeBatch.mutate(
            { conversationId, messageIds: selected },
            {
                onSuccess: () => {
                    setConfirmingDelete(false)
                    setSelected(null)
                },
            },
        )
    }
    // общий закреп в группе снимает админ, в личном - любой из двоих; личный - только владелец
    const canUnpin = (p: PinnedMessage) =>
        p.scope === 'me' || !isGroup || chat.data!.my_role === 'admin'

    // закреп общий (виден обоим) - тогда в окне открепления есть флажок «...также для собеседника»
    const sharedPin = (messageId: string) =>
        pinList.some((p) => p.message.id === messageId && p.scope === 'all')

    // кнопка в меню сообщения и крестик плашки: в группе админ действует сразу,
    // в личном чате - через подтверждение
    function togglePin(messageId: string) {
        const pinned = pinList.some((p) => p.message.id === messageId)
        // в «Избранном» спрашивать не о ком
        if (!isGroup && !chat.data!.is_saved) {
            setPinDialog({ mode: pinned ? 'unpin' : 'pin', messageId })
        } else {
            pin.mutate({ messageId, scope: 'all', pinned: !pinned })
        }
    }

    // forBoth - отмечен ли флажок; сервер при all снимает и мой личный закреп,
    // при me оставляет общий закреп собеседнику его личным
    function confirmPin(forBoth: boolean) {
        if (pinDialog) {
            pin.mutate({
                messageId: pinDialog.messageId,
                scope: forBoth ? 'all' : 'me',
                pinned: pinDialog.mode === 'pin',
            })
        }
        setPinDialog(null)
    }

    const replyTarget: ReplyTarget | null = reply
        ? {
              id: reply.id,
              author: reply.sender_id === myId ? 'Вы' : (replyAuthor.data?.display_name ?? ''),
              text: reply.content ?? '',
          }
        : draftReply

    return (
        <div className={styles.root}>
            <ChatHeader
                chat={chat.data}
                onOpenInfo={chat.data.is_saved ? undefined : () => openInfo(chat.data!)}
                actions={
                    <IconButton
                        icon="menu-dot"
                        aria-label="Действия с чатом"
                        aria-haspopup="menu"
                        isActive={!!menuAt}
                        // нажатие не должно считаться кликом «снаружи» меню: кнопка сама открывает и закрывает его
                        onMouseDown={(event) => event.stopPropagation()}
                        onClick={(event) => {
                            const rect = event.currentTarget.getBoundingClientRect()
                            setMenuAt((current) =>
                                current
                                    ? null
                                    : { x: rect.right, y: rect.bottom + 8, align: 'end' },
                            )
                        }}
                    />
                }
            />
            <ChatActionsMenu chat={chat.data} position={menuAt} onClose={() => setMenuAt(null)} />
            <PinnedBar
                pins={pinList}
                canUnpin={canUnpin}
                onJump={(id) => setJumpRequest((r) => ({ id, n: (r?.n ?? 0) + 1 }))}
                onUnpin={(p) => togglePin(p.message.id)}
            />
            {/* окно монтируется при открытии: флажок каждый раз снова выключен */}
            {pinDialog && (
                <PinConfirmModal
                    mode={pinDialog.mode}
                    peerId={peerId}
                    withPeerOption={pinDialog.mode === 'pin' || sharedPin(pinDialog.messageId)}
                    onClose={() => setPinDialog(null)}
                    onConfirm={confirmPin}
                />
            )}

            {history.isPending ? (
                <MessagesSkeleton />
            ) : history.isError && messages.length === 0 ? (
                <div className={styles.state}>
                    <p className={styles.stateText}>Не удалось загрузить сообщения</p>
                    <Button
                        variant="secondary"
                        className={styles.retry}
                        onClick={() => void history.refetch()}
                    >
                        Повторить
                    </Button>
                </div>
            ) : messages.length === 0 ? (
                <div className={styles.state}>
                    <Icon name="shape-message" size={67} color="var(--color-text)" />
                    <p className={styles.stateText}>Сообщений пока нет</p>
                </div>
            ) : (
                <MessageList
                    chat={chat.data}
                    messages={messages}
                    myId={myId}
                    readBoundary={readBoundary(messages, members.data, myId)}
                    hasOlder={history.hasNextPage}
                    loadingOlder={history.isFetchingNextPage}
                    loadOlder={loadOlder}
                    hasNewer={history.hasNewer}
                    loadingNewer={history.isFetchingPreviousPage}
                    loadNewer={loadNewer}
                    onReturnToLatest={returnToLatest}
                    firstUnreadId={divider?.id ?? null}
                    onSeen={setSeenId}
                    findMessage={findMessage}
                    onReply={(message) => {
                        setEditing(null)
                        setDraftReply(null)
                        setReply(message)
                    }}
                    onEdit={(message) => {
                        setReply(null)
                        setDraftReply(null)
                        setEditing(message)
                    }}
                    onDelete={(message) => remove.mutate(message)}
                    pins={pinList}
                    onPin={(message) => togglePin(message.id)}
                    onShowReaders={(message) => setReadersOf(message.id)}
                    onToggleReaction={(message, emoji, active) =>
                        toggleReaction.mutate({ messageId: message.id, emoji, active })
                    }
                    onShowReactions={(message, initialEmoji) =>
                        setReactorsOf({ messageId: message.id, initialEmoji })
                    }
                    jumpRequest={jumpRequest}
                    onForward={(message) => {
                        setReply(null)
                        setEditing(null)
                        setSelected([message.id])
                    }}
                    selected={selected}
                    onToggleSelect={toggleSelect}
                />
            )}

            {forwarding && selected && (
                <ForwardModal
                    messageIds={selected}
                    onClose={() => setForwarding(false)}
                    onDone={() => {
                        setForwarding(false)
                        setSelected(null)
                    }}
                />
            )}

            <ConfirmModal
                open={confirmingDelete && !!selected && selected.length > 0}
                onClose={() => setConfirmingDelete(false)}
                onConfirm={confirmDeleteSelected}
                title="Удалить сообщения?"
                confirmLabel="Удалить"
                danger
                pending={removeBatch.isPending}
            >
                {selected &&
                    `Будет удалено ${selected.length} ${plural(selected.length, ['сообщение', 'сообщения', 'сообщений'])}. Это действие нельзя отменить.`}
            </ConfirmModal>

            <ReadersModal
                conversationId={conversationId}
                messageId={readersOf}
                onClose={() => setReadersOf(null)}
            />

            <ReactorsModal
                reactions={
                    reactorsOf
                        ? (messages.find((m) => m.id === reactorsOf.messageId)?.reactions ?? [])
                        : null
                }
                initialEmoji={reactorsOf?.initialEmoji}
                onClose={() => setReactorsOf(null)}
            />

            <div className={styles.composer}>
                {selected !== null ? (
                    <SelectionBar
                        count={selected.length}
                        canDelete={canDeleteSelected}
                        onCopy={copySelected}
                        onDelete={() => setConfirmingDelete(true)}
                        onForward={() => setForwarding(true)}
                        onCancel={() => setSelected(null)}
                    />
                ) : canWrite || !peerId ? (
                    <MessageComposer
                        key={editing?.id ?? 'new'}
                        conversationId={conversationId}
                        myId={myId}
                        replyTo={replyTarget}
                        editing={editing}
                        onCancel={() => {
                            if (editing) {
                                setEditing(null)
                                return
                            }
                            setReply(null)
                            setDraftReply(null)
                        }}
                    />
                ) : (
                    <BlockedNotice peerId={peerId} />
                )}
            </div>
        </div>
    )
}
