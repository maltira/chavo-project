import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import { PhotoPicker } from './PhotoPicker'
import styles from './PhotoActions.module.scss'

interface Props {
    hasPhoto: boolean
    onPick: (photo: Blob) => void
    onRemove?: () => void
    removeLabel?: string
    disabled?: boolean
}

// пара компактных кнопок под аватаром: выбрать/сменить и удалить фото
export function PhotoActions({
    hasPhoto,
    onPick,
    onRemove,
    removeLabel = 'Удалить фото',
    disabled = false,
}: Props) {
    return (
        <div className={styles.actions}>
            <PhotoPicker onPick={onPick}>
                {(open) => (
                    <button
                        type="button"
                        className={styles.button}
                        disabled={disabled}
                        onClick={open}
                    >
                        <Icon name={hasPhoto ? 'edit' : 'add'} size={16} color="currentColor" />
                        <span>{hasPhoto ? 'Сменить фото' : 'Выбрать фото'}</span>
                    </button>
                )}
            </PhotoPicker>
            {hasPhoto && onRemove && (
                <button
                    type="button"
                    className={cn(styles.button, styles.danger)}
                    disabled={disabled}
                    onClick={onRemove}
                >
                    <Icon name="trash" size={16} color="currentColor" />
                    <span>{removeLabel}</span>
                </button>
            )}
        </div>
    )
}
