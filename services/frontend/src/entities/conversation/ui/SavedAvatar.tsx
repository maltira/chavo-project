import { Icon } from '@/shared/ui'

import styles from './SavedAvatar.module.scss'

// название чата с самим собой
export const SAVED_NAME = 'Избранное'

// «Избранное» вместо аватара: звезда на акцентном круге
export function SavedAvatar({ size = 44 }: { size?: number }) {
    return (
        <span className={styles.root} style={{ width: size, height: size }} aria-hidden="true">
            <Icon name="star" size={Math.round(size * 0.5)} color="currentColor" />
        </span>
    )
}
