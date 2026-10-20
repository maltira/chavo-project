import { useBlockStatus } from '@/entities/user'
import { useBlockUser } from '@/features/block-user'
import { Icon } from '@/shared/ui'

import styles from './BlockedNotice.module.scss'

interface Props {
    peerId: string
}

// плашка вместо поля ввода, когда писать нельзя: собеседник ограничил отправку или я его заблокировал
export function BlockedNotice({ peerId }: Props) {
    const status = useBlockStatus(peerId)
    const { unblock, pending } = useBlockUser(peerId)
    const byMe = status.data?.blocked_by_me ?? false

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

// можно ли писать собеседнику: пока статус не загружен, поле ввода показываем
export function useCanWrite(peerId: string | null | undefined) {
    const status = useBlockStatus(peerId)
    return !status.data || (!status.data.blocked_by_me && !status.data.blocked_by_them)
}
