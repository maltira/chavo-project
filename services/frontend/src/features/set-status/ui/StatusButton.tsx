import { cn } from '@/shared/lib'
import { Icon, Status } from '@/shared/ui'

import { ME } from '../model/target'
import type { CustomTarget } from '../model/target'
import { useTargetStatus } from '../model/useTargetStatus'
import styles from './StatusButton.module.scss'

interface Props {
    // открыть редактор статуса
    onEdit: () => void
    // во всю ширину (в меню профиля); по умолчанию - по размеру плашки
    block?: boolean
    // по умолчанию - мой статус
    target?: CustomTarget
}

// статус (мой или группы) в профиле и настройках: по нажатию - редактор; без статуса - «Добавить статус»
export function StatusButton({ onEdit, block = false, target = ME }: Props) {
    const status = useTargetStatus(target)
    // истёкший статус сервер ещё может отдать до перезагрузки профиля
    const active = status && (!status.expires_at || new Date(status.expires_at) > new Date())

    return (
        <button
            type="button"
            className={cn(styles.button, block && styles.block)}
            aria-label={active ? `Статус: ${status.text}. Изменить` : 'Установить статус'}
            onClick={onEdit}
        >
            {active ? (
                <Status
                    icon={status.icon}
                    text={status.text}
                    background={status.background}
                    foreground={status.foreground}
                    expiresAt={status.expires_at}
                />
            ) : (
                <span className={styles.empty}>
                    <Icon
                        name="shape-plus"
                        size={24}
                        color="inherit"
                        className={styles.emptyIcon}
                    />
                    <span className={styles.emptyText}>Добавить статус</span>
                </span>
            )}
        </button>
    )
}
