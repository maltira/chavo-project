import { dismiss, useNotifications } from '@/shared/lib'
import { Notification } from '@/shared/ui'

import styles from './NotificationsHost.module.scss'

// стопка уведомлений в правом нижнем углу: новое появляется снизу
export function NotificationsHost() {
    const items = useNotifications((state) => state.items)

    return (
        <div className={styles.host} aria-live="polite">
            {items.map((item) => (
                <Notification
                    key={item.id}
                    title={item.title}
                    description={item.description}
                    icon={item.icon}
                    onClose={() => dismiss(item.id)}
                />
            ))}
        </div>
    )
}
