import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { settingsKey, updateSettings, useSettings } from '@/entities/user'
import type { Settings } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Button, SkeletonRow } from '@/shared/ui'

import styles from './PrivacyScreen.module.scss'
import common from './SettingsModal.module.scss'

// конфиденциальность: кто зовёт в группы, кто видит, что я в сети, и чьё имя видно при пересылке
export function PrivacyScreen({ onSaved }: { onSaved: () => void }) {
    const settings = useSettings()

    if (!settings.data) {
        return (
            <div className={common.body}>
                <SkeletonRow />
                <SkeletonRow />
                <SkeletonRow />
            </div>
        )
    }
    return <PrivacyForm initial={settings.data} onSaved={onSaved} />
}

function PrivacyForm({ initial, onSaved }: { initial: Settings; onSaved: () => void }) {
    const queryClient = useQueryClient()
    const [invites, setInvites] = useState(initial.allow_group_invites)
    const [online, setOnline] = useState(initial.show_online_status)
    // «Все» - имя видно, «Никто» - в пересланных «скрытый автор»
    const [forwardName, setForwardName] = useState(!initial.hide_forward_author)
    const changed =
        invites !== initial.allow_group_invites ||
        online !== initial.show_online_status ||
        forwardName === initial.hide_forward_author

    const save = useMutation({
        mutationFn: () =>
            updateSettings({
                allow_group_invites: invites,
                show_online_status: online,
                hide_forward_author: !forwardName,
            }),
        onSuccess: (saved) => {
            queryClient.setQueryData(settingsKey, saved)
            // от show_online_status зависят мои online и last_seen_at в профиле
            void queryClient.invalidateQueries({ queryKey: ['me'] })
            notify({ icon: 'check', title: 'Настройки сохранены' })
            onSaved()
        },
        onError: (error) =>
            notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) }),
    })

    return (
        <div className={common.body}>
            <div className={common.card}>
                <Choice
                    question="Кто может добавлять меня в беседы?"
                    value={invites}
                    onChange={setInvites}
                />
                <Choice
                    question="Кто видит мой статус в сети?"
                    value={online}
                    onChange={setOnline}
                />
                <Choice
                    question="Кто видит моё имя при пересылке?"
                    value={forwardName}
                    onChange={setForwardName}
                />
            </div>
            <p className={common.hint}>
                Если скрыть статус в сети, другие не увидят, когда вы были онлайн. Если скрыть имя
                при пересылке, ваши сообщения будут пересылаться от «скрытого автора».
            </p>

            <Button
                variant="secondary"
                className={common.footer}
                disabled={!changed || save.isPending}
                onClick={() => save.mutate()}
            >
                Сохранить изменения
            </Button>
        </div>
    )
}

interface ChoiceProps {
    question: string
    // true - все, false - никто
    value: boolean
    onChange: (value: boolean) => void
}

// вопрос и кнопка-значение: нажатие переключает «Все» и «Никто»
function Choice({ question, value, onChange }: ChoiceProps) {
    return (
        <div className={styles.row}>
            <span className={styles.question}>{question}</span>
            <button
                type="button"
                className={styles.value}
                aria-label={`${question} Сейчас: ${value ? 'все' : 'никто'}`}
                onClick={() => onChange(!value)}
            >
                {value ? 'Все' : 'Никто'}
            </button>
        </div>
    )
}
