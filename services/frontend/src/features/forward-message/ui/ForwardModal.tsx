import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
    SAVED_NAME,
    SavedAvatar,
    useArchivedConversations,
    useConversations,
} from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { MAX_FORWARD_CHATS } from '@/entities/message'
import { userKey, UserAvatar } from '@/entities/user'
import type { Profile } from '@/entities/user'
import { Button, Checkbox, Input, Modal, Switch } from '@/shared/ui'

import { useForward } from '../model/useForward'
import styles from './ForwardModal.module.scss'

interface Props {
    messageIds: string[]
    onClose: () => void
    // переслано - выйти из режима выбора
    onDone: () => void
}

// «Избранное» до первого сообщения - отдельный пункт: его создаст сервер
const SAVED = 'saved'

interface Target {
    key: string
    name: string
    chat: ConversationSummary | null
}

// куда переслать: мои чаты (и архивные) и «Избранное», до 5 сразу
export function ForwardModal({ messageIds, onClose, onDone }: Props) {
    const queryClient = useQueryClient()
    const main = useConversations()
    const archived = useArchivedConversations()
    const [query, setQuery] = useState('')
    const [chosen, setChosen] = useState<string[]>([])
    const [hideAuthor, setHideAuthor] = useState(false)
    const forward = useForward(onDone)

    // имя собеседника - из уже загруженного профиля (его подгрузил список чатов)
    const nameOf = (chat: ConversationSummary) =>
        chat.is_saved
            ? SAVED_NAME
            : chat.conversation_type === 'group'
              ? (chat.name ?? 'Группа')
              : (queryClient.getQueryData<Profile>(userKey(chat.peer_id ?? ''))?.display_name ?? '')

    const chats = [...(main.data ?? []), ...(archived.data ?? [])]
    const saved = chats.find((c) => c.is_saved)
    const targets: Target[] = [
        saved
            ? { key: saved.id, name: SAVED_NAME, chat: saved }
            : { key: SAVED, name: SAVED_NAME, chat: null },
        ...chats.filter((c) => !c.is_saved).map((c) => ({ key: c.id, name: nameOf(c), chat: c })),
    ]
    const q = query.trim().toLowerCase()
    const shown = q ? targets.filter((t) => t.name.toLowerCase().includes(q)) : targets
    const full = chosen.length >= MAX_FORWARD_CHATS

    const toggle = (key: string) =>
        setChosen((keys) => (keys.includes(key) ? keys.filter((k) => k !== key) : [...keys, key]))

    function send() {
        forward.mutate({
            message_ids: messageIds,
            conversation_ids: chosen.filter((k) => k !== SAVED),
            to_saved: chosen.includes(SAVED),
            hide_author: hideAuthor,
        })
    }

    return (
        <Modal open onClose={onClose} title="Переслать" titleAlign="start" width={471}>
            <Input
                placeholder="Поиск по чатам"
                aria-label="Поиск по чатам"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
            />
            <div className={styles.list}>
                {shown.map((target) => {
                    const checked = chosen.includes(target.key)
                    return (
                        <Checkbox
                            key={target.key}
                            className={styles.row}
                            checked={checked}
                            disabled={!checked && full}
                            onChange={() => toggle(target.key)}
                        >
                            <span className={styles.who}>
                                {!target.chat || target.chat.is_saved ? (
                                    <SavedAvatar size={40} />
                                ) : (
                                    <TargetAvatar chat={target.chat} />
                                )}
                                <span className={styles.name}>{target.name || ' '}</span>
                            </span>
                        </Checkbox>
                    )
                })}
                {shown.length === 0 && <p className={styles.hint}>Ничего не найдено</p>}
            </div>
            <label className={styles.option}>
                <span>Без указания автора</span>
                <Switch
                    aria-label="Без указания автора"
                    checked={hideAuthor}
                    onChange={(event) => setHideAuthor(event.target.checked)}
                />
            </label>
            <Button
                variant="secondary"
                disabled={chosen.length === 0 || forward.isPending}
                onClick={send}
            >
                {chosen.length > 0 ? `Переслать (${chosen.length})` : 'Переслать'}
            </Button>
        </Modal>
    )
}

// аватар группы или собеседника
function TargetAvatar({ chat }: { chat: ConversationSummary }) {
    const queryClient = useQueryClient()
    const peer =
        chat.conversation_type === 'direct'
            ? queryClient.getQueryData<Profile>(userKey(chat.peer_id ?? ''))
            : undefined
    return (
        <UserAvatar
            src={peer ? peer.avatar_url : chat.avatar_url}
            shape={peer ? peer.avatar_shape : chat.avatar_shape}
            size={40}
        />
    )
}
