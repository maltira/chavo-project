import { useState } from 'react'

import type { ConversationSummary } from '@/entities/conversation'
import { chatLinkText } from '@/shared/config'
import { ConfirmModal, TextLink } from '@/shared/ui'

import { copyChatLink, inviteCodeOf, useRegenerateInvite } from '../model/useInviteLink'
import styles from './ChatLink.module.scss'

interface Props {
    chat: ConversationSummary
}

// значение строки «Ссылка на чат»: по нажатию копируется; у закрытой группы админ может её сбросить
export function ChatLink({ chat }: Props) {
    const code = inviteCodeOf(chat)
    const regenerate = useRegenerateInvite(chat.id)
    const [confirm, setConfirm] = useState(false)
    const canReset = chat.visibility === 'private' && chat.my_role === 'admin'

    if (!code) {
        return null
    }

    return (
        <span className={styles.root}>
            <button
                type="button"
                className={styles.link}
                aria-label="Скопировать ссылку на чат"
                title="Скопировать"
                onClick={() => void copyChatLink(code)}
            >
                {chatLinkText(code)}
            </button>
            {canReset && (
                <TextLink
                    as="button"
                    className={styles.reset}
                    disabled={regenerate.isPending}
                    onClick={() => setConfirm(true)}
                >
                    Сбросить ссылку
                </TextLink>
            )}

            <ConfirmModal
                open={confirm}
                onClose={() => setConfirm(false)}
                onConfirm={() => {
                    setConfirm(false)
                    regenerate.mutate()
                }}
                confirmLabel="Сбросить"
                danger
                pending={regenerate.isPending}
            >
                Прежняя ссылка перестанет работать, новая появится здесь же
            </ConfirmModal>
        </span>
    )
}
