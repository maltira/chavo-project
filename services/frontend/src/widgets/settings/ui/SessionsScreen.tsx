import { useState } from 'react'

import { currentSessionId, useSessions } from '@/entities/session'
import type { Session } from '@/entities/session'
import { useTerminateSessions } from '@/features/terminate-session'
import { formatChatTime } from '@/shared/lib'
import { Button, ConfirmModal, Icon, SkeletonRow } from '@/shared/ui'

import styles from './SessionsScreen.module.scss'
import common from './SettingsModal.module.scss'

const deviceName = (session: Session) =>
    session.device_name || session.user_agent || 'Неизвестное устройство'

// активные сеансы: текущий отдельно, остальные можно завершить по одному или все
export function SessionsScreen() {
    const sessions = useSessions()
    const terminate = useTerminateSessions()
    // что подтверждаем: один сеанс или все чужие
    const [confirm, setConfirm] = useState<Session | 'others' | null>(null)
    const currentId = currentSessionId()

    if (!sessions.data) {
        return (
            <div className={common.body}>
                <SkeletonRow />
                <SkeletonRow />
                <SkeletonRow />
            </div>
        )
    }

    const current = sessions.data.find((s) => s.id === currentId)
    const others = sessions.data.filter((s) => s.id !== currentId)

    return (
        <div className={common.body}>
            {current && (
                <div className={styles.current}>
                    <span className={styles.star}>
                        <Icon name="star" size={22} color="var(--color-accent)" />
                    </span>
                    <span className={styles.texts}>
                        <span className={styles.device}>{deviceName(current)}</span>
                        <span className={styles.meta}>Текущий сеанс</span>
                    </span>
                </div>
            )}

            {others.length > 0 ? (
                <>
                    <Button
                        variant="secondary"
                        disabled={terminate.isPending}
                        onClick={() => setConfirm('others')}
                    >
                        Завершить все, кроме текущего
                    </Button>
                    <div className={common.card}>
                        {others.map((session) => (
                            <button
                                key={session.id}
                                type="button"
                                className={styles.session}
                                aria-label={`Завершить сеанс: ${deviceName(session)}`}
                                onClick={() => setConfirm(session)}
                            >
                                <span className={styles.texts}>
                                    <span className={styles.device}>{deviceName(session)}</span>
                                    {session.ip_address && (
                                        <span className={styles.meta}>{session.ip_address}</span>
                                    )}
                                </span>
                                <span className={styles.time}>
                                    {formatChatTime(session.created_at)}
                                </span>
                            </button>
                        ))}
                    </div>
                </>
            ) : (
                <p className={common.hint}>Других активных сеансов нет</p>
            )}

            <ConfirmModal
                open={confirm !== null}
                onClose={() => setConfirm(null)}
                onConfirm={() => {
                    const ids =
                        confirm === 'others' ? others.map((s) => s.id) : confirm ? [confirm.id] : []
                    setConfirm(null)
                    terminate.mutate(ids)
                }}
                confirmLabel="Завершить"
                danger
            >
                {confirm === 'others'
                    ? 'На всех других устройствах придётся войти заново'
                    : `На устройстве «${confirm ? deviceName(confirm) : ''}» придётся войти заново`}
            </ConfirmModal>
        </div>
    )
}
