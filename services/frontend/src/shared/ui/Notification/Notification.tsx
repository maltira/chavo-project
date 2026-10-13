import { Icon } from '../Icon'
import { IconButton } from '../IconButton'
import styles from './Notification.module.scss'

interface Props {
    title: string
    description?: string
    icon?: string
    onClose?: () => void
}

export function Notification({ title, description, icon = 'warn', onClose }: Props) {
    return (
        <div role="status" className={styles.root}>
            <div className={styles.content}>
                <IconButton
                    icon={icon}
                    aria-label={title}
                    aria-hidden="true"
                    tabIndex={-1}
                    className={styles.icon}
                />
                <div className={styles.text}>
                    <p className={styles.title}>{title}</p>
                    {description && <p className={styles.description}>{description}</p>}
                </div>
            </div>

            {onClose && (
                <button
                    type="button"
                    className={styles.close}
                    aria-label="Закрыть"
                    onClick={onClose}
                >
                    <Icon name="close" size={24} color="inherit" />
                </button>
            )}
        </div>
    )
}
