import { useBlockStatus, useUser } from '@/entities/user'
import { useBlockUser } from '@/features/block-user'
import { Icon } from '@/shared/ui'

import styles from './BlockedNotice.module.scss'

interface Props {
    peerId: string
}

// плашка вместо поля ввода, когда писать нельзя: удалённый аккаунт, собеседник ограничил отправку или я его заблокировал
export function BlockedNotice({ peerId }: Props) {
    const peer = useUser(peerId)
    const status = useBlockStatus(peerId)
    const { unblock, pending } = useBlockUser(peerId)
    const deleted = peer.data?.is_deleted ?? false
    const byMe = status.data?.blocked_by_me ?? false

    if (deleted) {
        return (
            <div className={styles.root} role="status">
                <Icon name="shape-user" size={32} color="inherit" className={styles.icon} />
                <span className={styles.text}>Аккаунт удалён — отправка сообщений недоступна</span>
            </div>
        )
    }

    return (
        <div className={styles.root} role="status">
            <Icon name="shape-block" size={32} color="inherit" className={styles.icon} />
            <span className={styles.text}>
                {byMe
                    ? 'Вы заблокировали пользователя'
                    : 'Пользователь ограничил отправку сообщений'}
            </span>
            {byMe && (
                <button
                    type="button"
                    className={styles.unblock}
                    disabled={pending}
                    onClick={() => unblock.mutate()}
                >
                    Разблокировать
                </button>
            )}
        </div>
    )
}

// можно ли писать собеседнику: нельзя, если аккаунт удалён или есть блокировка; пока данные не загружены - поле показываем
export function useCanWrite(peerId: string | null | undefined) {
    const peer = useUser(peerId)
    const status = useBlockStatus(peerId)
    if (peer.data?.is_deleted) {
        return false
    }
    return !status.data || (!status.data.blocked_by_me && !status.data.blocked_by_them)
}
