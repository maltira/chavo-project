import { cn } from '@/shared/lib'

import { Icon } from '../Icon'
import { IconButton } from '../IconButton'
import styles from './Notification.module.scss'

interface Props {
    title: string
    description?: string
    icon?: string
    onClose?: () => void
    // клик по тексту уведомления; без него уведомление не кликабельно
    onClick?: () => void
}

export function Notification({ title, description, icon = 'warn', onClose, onClick }: Props) {
    const Content = onClick ? 'button' : 'div'
    return (
        <div role="status" className={styles.root}>
            <Content
                className={cn(styles.content, onClick && styles.clickable)}
                {...(onClick ? { type: 'button' as const, onClick } : {})}
            >
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
            </Content>

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
