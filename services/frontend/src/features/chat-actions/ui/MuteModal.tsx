import { muteDurationLabel, useCustomization } from '@/entities/customization'
import { Button, Modal } from '@/shared/ui'

import styles from './MuteModal.module.scss'

interface Props {
    open: boolean
    onClose: () => void
    onPick: (duration: string) => void
    pending?: boolean
}

// выбор срока «без звука»: сроки - из каталога сервера
export function MuteModal({ open, onClose, onPick, pending = false }: Props) {
    const catalog = useCustomization()
    const durations = catalog.data?.mute_durations ?? []

    return (
        <Modal open={open} onClose={onClose} title="Без звука" titleAlign="start" width={424}>
            <p className={styles.text}>Уведомления этого чата не будут приходить</p>
            <div className={styles.options}>
                {durations.map((key) => (
                    <Button
                        key={key}
                        variant="secondary"
                        disabled={pending}
                        onClick={() => onPick(key)}
                    >
                        {muteDurationLabel(key)}
                    </Button>
                ))}
            </div>
        </Modal>
    )
}
