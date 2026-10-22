import { Button } from '@/shared/ui'

import styles from './SelectionBar.module.scss'

interface Props {
    count: number
    onForward: () => void
    onCancel: () => void
}

// панель режима выбора вместо поля ввода: сколько отмечено, «Переслать» и «Отмена» (или Esc)
export function SelectionBar({ count, onForward, onCancel }: Props) {
    return (
        <div className={styles.root} role="toolbar" aria-label="Выбранные сообщения">
            <span className={styles.count}>Выбрано: {count}</span>
            <Button variant="secondary" className={styles.button} onClick={onCancel}>
                Отмена
            </Button>
            <Button className={styles.button} disabled={count === 0} onClick={onForward}>
                Переслать
            </Button>
        </div>
    )
}
