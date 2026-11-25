import { useUser } from '@/entities/user'
import { Button, Modal } from '@/shared/ui'

import styles from './DeleteDirectModal.module.scss'

interface Props {
    open: boolean
    peerId: string | null
    pending: boolean
    onClose: () => void
    // forEveryone - у обоих, иначе только у себя
    onDelete: (forEveryone: boolean) => void
}

// удаление личного чата: только у себя (собеседник ничего не заметит) или у обоих
export function DeleteDirectModal({ open, peerId, pending, onClose, onDelete }: Props) {
    const peer = useUser(open ? peerId : null)
    const name = peer.data?.display_name

    return (
        <Modal open={open} onClose={onClose} title="Удалить чат?" titleAlign="start" width={424}>
            <p className={styles.text}>
                «Только у меня» - история пропадёт у вас, а если {name ?? 'собеседник'} напишет
                снова, чат вернётся с новыми сообщениями. «У обоих» - история удалится у вас обоих
                навсегда
            </p>
            <div className={styles.actions}>
                <Button variant="danger" disabled={pending} onClick={() => onDelete(true)}>
                    У меня и у {name ?? 'собеседника'}
                </Button>
                <Button variant="secondary" disabled={pending} onClick={() => onDelete(false)}>
                    Только у меня
                </Button>
            </div>
        </Modal>
    )
}
