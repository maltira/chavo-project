import { useState } from 'react'

import { isMuted } from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { markRead } from '@/entities/message'
import { ConfirmModal, ContextMenu, MenuItem } from '@/shared/ui'
import type { MenuPosition } from '@/shared/ui'

import { useChatSettings } from '../model/useChatSettings'
import { useLeaveChat } from '../model/useLeaveChat'
import { useOpenChatInfo } from '../model/useOpenChatInfo'
import { DeleteDirectModal } from './DeleteDirectModal'
import { MuteModal } from './MuteModal'

interface Props {
    chat: ConversationSummary
    position: MenuPosition | null
    onClose: () => void
}

// меню чата (правая кнопка в списке и «⋮» в шапке): сведения, прочитать, закреп, «без звука», архив,
// покинуть/удалить группу
export function ChatActionsMenu({ chat, position, onClose }: Props) {
    const openInfo = useOpenChatInfo()
    const leave = useLeaveChat()
    const settings = useChatSettings(chat.id)
    const [confirm, setConfirm] = useState(false)
    const [mutePicker, setMutePicker] = useState(false)
    const muted = isMuted(chat)

    const isGroup = chat.conversation_type === 'group'
    const isAdmin = isGroup && chat.my_role === 'admin'
    const last = chat.last_message

    return (
        <>
            <ContextMenu position={position} onClose={onClose} aria-label="Действия с чатом">
                {/* у «Избранного» сведений нет: собеседника нет */}
                {!chat.is_saved && (
                    <MenuItem icon="settings" onClick={() => openInfo(chat)}>
                        Сведения о чате
                    </MenuItem>
                )}
                {chat.unread_count > 0 && last && (
                    <MenuItem
                        icon="check-double"
                        // счётчик в списке обновит событие о своём прочтении
                        onClick={() => void markRead(chat.id, last.id).catch(() => {})}
                    >
                        Прочитать чат
                    </MenuItem>
                )}
                <MenuItem
                    icon="pinned"
                    onClick={() => settings.mutate({ kind: 'pin', pinned: !chat.pinned_at })}
                >
                    {chat.pinned_at ? 'Открепить чат' : 'Закрепить чат'}
                </MenuItem>
                <MenuItem
                    icon={muted ? 'notif' : 'notif-off'}
                    onClick={() =>
                        muted ? settings.mutate({ kind: 'unmute' }) : setMutePicker(true)
                    }
                >
                    {muted ? 'Включить звук' : 'Без звука'}
                </MenuItem>
                <MenuItem
                    icon="archive"
                    onClick={() =>
                        settings.mutate({ kind: 'archive', archived: !chat.archived_at })
                    }
                >
                    {chat.archived_at ? 'Вернуть из архива' : 'В архив'}
                </MenuItem>
                {isGroup ? (
                    <MenuItem
                        icon={isAdmin ? 'trash' : 'exit'}
                        danger
                        onClick={() => setConfirm(true)}
                    >
                        {isAdmin ? 'Удалить чат' : 'Покинуть чат'}
                    </MenuItem>
                ) : (
                    <MenuItem icon="trash" danger onClick={() => setConfirm(true)}>
                        Удалить чат
                    </MenuItem>
                )}
            </ContextMenu>

            {/* каталог сроков грузится, только когда окно открыли */}
            {mutePicker && (
                <MuteModal
                    open
                    onClose={() => setMutePicker(false)}
                    pending={settings.isPending}
                    onPick={(duration) =>
                        settings.mutate(
                            { kind: 'mute', duration },
                            { onSuccess: () => setMutePicker(false) },
                        )
                    }
                />
            )}

            {!isGroup && !chat.is_saved && (
                <DeleteDirectModal
                    open={confirm}
                    peerId={chat.peer_id}
                    pending={leave.isPending}
                    onClose={() => setConfirm(false)}
                    onDelete={(forEveryone) => {
                        setConfirm(false)
                        leave.mutate({ conversationId: chat.id, forEveryone })
                    }}
                />
            )}

            <ConfirmModal
                open={confirm && (isGroup || chat.is_saved)}
                onClose={() => setConfirm(false)}
                onConfirm={() => {
                    setConfirm(false)
                    // «Избранное» удаляется целиком: в нём никого, кроме меня
                    leave.mutate({ conversationId: chat.id, forEveryone: chat.is_saved })
                }}
                confirmLabel={isAdmin || chat.is_saved ? 'Удалить' : 'Покинуть'}
                danger
                pending={leave.isPending}
            >
                {chat.is_saved
                    ? 'Все сообщения в «Избранном» будут удалены'
                    : isAdmin
                      ? 'Группа и вся её история будут удалены у всех участников'
                      : 'Чат и его история пропадут из вашего списка'}
            </ConfirmModal>
        </>
    )
}
