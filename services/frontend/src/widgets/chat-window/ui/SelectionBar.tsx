import { Button, IconButton } from '@/shared/ui'

import styles from './SelectionBar.module.scss'

interface Props {
    count: number
    canDelete: boolean
    onCopy: () => void
    onDelete: () => void
    onForward: () => void
    onCancel: () => void
}

// панель режима выбора вместо поля ввода: счётчик, копирование, удаление, «Переслать» и «Отмена»
export function SelectionBar({
    count,
    canDelete,
    onCopy,
    onDelete,
    onForward,
    onCancel,
}: Props) {
    return (
        <div className={styles.root} role="toolbar" aria-label="Выбранные сообщения">
            <span className={styles.count}>Выбрано: {count}</span>
            <IconButton
                icon="copy"
                aria-label="Копировать выбранные"
                disabled={count === 0}
                onClick={onCopy}
            />
            {canDelete && (
                <IconButton
                    icon="trash"
                    aria-label="Удалить выбранные"
                    className={styles.danger}
                    disabled={count === 0}
                    onClick={onDelete}
                />
            )}
            <Button variant="secondary" className={styles.button} onClick={onCancel}>
                Отмена
            </Button>
            <Button className={styles.button} disabled={count === 0} onClick={onForward}>
                Переслать
            </Button>
        </div>
    )
}
