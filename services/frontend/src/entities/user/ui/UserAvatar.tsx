import { Avatar, AvatarShape } from '@/shared/ui'

import type { AvatarShape as Shape } from '../model/types'
import styles from './UserAvatar.module.scss'

interface Props {
    src?: string | null
    shape?: Shape | null
    size?: number
    online?: boolean
    className?: string
}

// аватар с фигурой позади и точкой «в сети»
export function UserAvatar({ src, shape, size = 44, online = false, className }: Props) {
    return (
        <span className={[styles.root, className].filter(Boolean).join(' ')}>
            <AvatarShape shape={shape?.shape} color={shape?.color}>
                <Avatar src={src} size={size} alt="" />
            </AvatarShape>

            <span
                className={styles.online}
                data-online={online}
                role={online ? 'img' : undefined}
                aria-label={online ? 'в сети' : undefined}
                aria-hidden={!online}
            />
        </span>
    )
}
