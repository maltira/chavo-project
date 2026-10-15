import type { ReactNode } from 'react'

import { Button } from '../Button'
import { Modal } from '../Modal'
import styles from './ConfirmModal.module.scss'

interface Props {
    open: boolean
    onClose: () => void
    onConfirm: () => void
    title?: string
    children?: ReactNode
    confirmLabel?: string
    cancelLabel?: string
    // подтверждаемое действие опасное (блокировка, удаление): кнопка красная
    danger?: boolean
    pending?: boolean
}

// «Вы уверены?» перед необратимыми действиями
export function ConfirmModal({
    open,
    onClose,
    onConfirm,
    title = 'Вы уверены?',
    children,
    confirmLabel = 'Да',
    cancelLabel = 'Отмена',
    danger = false,
    pending = false,
}: Props) {
    return (
        <Modal open={open} onClose={onClose} title={title} titleAlign="start" width={424}>
            {children && <p className={styles.text}>{children}</p>}
            <div className={styles.actions}>
                <Button
                    variant={danger ? 'danger' : 'primary'}
                    disabled={pending}
                    onClick={onConfirm}
                >
                    {confirmLabel}
                </Button>
                <Button variant="secondary" onClick={onClose}>
                    {cancelLabel}
                </Button>
            </div>
        </Modal>
    )
}
