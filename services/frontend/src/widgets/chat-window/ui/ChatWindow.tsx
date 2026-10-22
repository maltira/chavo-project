import { useCallback, useState } from 'react'

import { useConversation, useMembers } from '@/entities/conversation'
import { useMessages, usePins } from '@/entities/message'
import type { Message, PinnedMessage } from '@/entities/message'
import { useMe, useUser } from '@/entities/user'
import { MessageComposer } from '@/features/compose-message'
import type { ReplyTarget } from '@/features/compose-message'
import { ChatActionsMenu, useOpenChatInfo } from '@/features/chat-actions'
import { useDeleteMessage } from '@/features/delete-message'
import { useMarkRead } from '@/features/mark-read'
import { PinConfirmModal, usePinMessage } from '@/features/pin-message'
import { Button, Icon, IconButton } from '@/shared/ui'
import type { MenuPosition } from '@/shared/ui'

import { firstUnreadId, readBoundary } from '../lib/readState'
import { BlockedNotice, useCanWrite } from './BlockedNotice'
import { ChatHeader } from './ChatHeader'
import { HeaderSkeleton, MessagesSkeleton } from './ChatSkeleton'
import styles from './ChatWindow.module.scss'
import { MessageList } from './MessageList'
import { PinnedBar } from './PinnedBar'

interface Props {
    conversationId: string
}

// предел догрузки при переходе по цитате: дальше сообщение считается недоступным
const MAX_PAGES_TO_FIND = 20

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
    const openInfo = useOpenChatInfo()
    const pins = usePins(conversationId)
    const pin = usePinMessage(conversationId)
    // подтверждение закрепа или открепления в личном чате
    const [pinDialog, setPinDialog] = useState<{ mode: 'pin' | 'unpin'; messageId: string } | null>(
        null,
    )
    const [jumpRequest, setJumpRequest] = useState<{ id: string; n: number } | null>(null)

    // меню «⋮» в шапке - под кнопкой
    const [menuAt, setMenuAt] = useState<MenuPosition | null>(null)
    const [reply, setReply] = useState<Message | null>(null)
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

    const { fetchNextPage } = history
    const loadOlder = useCallback(
        () => void fetchNextPage({ cancelRefetch: false }),
        [fetchNextPage],
    )

    // переход по цитате к старому сообщению: догружаем историю страницами, пока не найдём
    const findMessage = useCallback(
        async (id: string) => {
            for (let page = 0; page < MAX_PAGES_TO_FIND; page++) {
                const result = await fetchNextPage({ cancelRefetch: false })
                const pages = result.data?.pages ?? []
                if (pages.some((p) => p.items.some((m) => m.id === id))) {
                    return true
                }
                if (!result.hasNextPage) {
                    return false
                }
            }
            return false
        },
        [fetchNextPage],
    )

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
    const pinList = pins.data ?? []
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
        : null

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
                    firstUnreadId={divider?.id ?? null}
                    onSeen={setSeenId}
                    findMessage={findMessage}
                    onReply={(message) => {
                        setEditing(null)
                        setReply(message)
                    }}
                    onEdit={(message) => {
                        setReply(null)
                        setEditing(message)
                    }}
                    onDelete={(message) => remove.mutate(message)}
                    pins={pinList}
                    onPin={(message) => togglePin(message.id)}
                    jumpRequest={jumpRequest}
                />
            )}

            <div className={styles.composer}>
                {canWrite || !peerId ? (
                    <MessageComposer
                        key={editing?.id ?? 'new'}
                        conversationId={conversationId}
                        myId={myId}
                        replyTo={replyTarget}
                        editing={editing}
                        onCancel={() => {
                            setReply(null)
                            setEditing(null)
                        }}
                    />
                ) : (
                    <BlockedNotice peerId={peerId} />
                )}
            </div>
        </div>
    )
}
