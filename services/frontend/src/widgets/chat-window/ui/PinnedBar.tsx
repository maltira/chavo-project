import { useState } from 'react'

import type { PinnedMessage } from '@/entities/message'
import { IconButton } from '@/shared/ui'

import styles from './PinnedBar.module.scss'

interface Props {
    // новые первыми
    pins: PinnedMessage[]
    canUnpin: (pin: PinnedMessage) => boolean
    onJump: (messageId: string) => void
    onUnpin: (pin: PinnedMessage) => void
}

// плашка закреплённых сообщений под шапкой: нажатие ведёт к сообщению и переключает на следующий закреп
export function PinnedBar({ pins, canUnpin, onJump, onUnpin }: Props) {
    const [index, setIndex] = useState(0)
    if (pins.length === 0) {
        return null
    }
    // открепили текущее - показываем то, что встало на его место
    const current = index % pins.length
    const pin = pins[current]!
    const label =
        pins.length > 1
            ? `Закреплённое сообщение ${current + 1} из ${pins.length}`
            : 'Закреплённое сообщение'

    return (
        <div className={styles.root}>
            <button
                type="button"
                className={styles.pin}
                onClick={() => {
                    onJump(pin.message.id)
                    setIndex(current + 1)
                }}
            >
                <span className={styles.title}>
                    {label}
                    {pin.scope === 'me' && <span className={styles.own}> · только у вас</span>}
                </span>
                <span className={styles.text}>{pin.message.content}</span>
            </button>
            {canUnpin(pin) && (
                <IconButton
                    icon="close"
                    size={32}
                    aria-label="Открепить"
                    onClick={() => onUnpin(pin)}
                />
            )}
        </div>
    )
}
