import { useSessions } from '@/entities/session'
import { useBlocked, useMe, useProfileCard, UserAvatar } from '@/entities/user'
import { useLogout } from '@/features/auth/logout'
import { MyStatusButton } from '@/features/set-status'
import { Button, Icon, MenuItem, Modal, Tag } from '@/shared/ui'

import type { Screen } from './SettingsModal'
import styles from './MainScreen.module.scss'
import common from './SettingsModal.module.scss'

interface Props {
    open: boolean
    onClose: () => void
    onNavigate: (screen: Screen) => void
}

// главный экран настроек: мой статус, профиль, разделы и выход
export function MainScreen({ open, onClose, onNavigate }: Props) {
    const me = useMe()
    const sessions = useSessions(open)
    const blocked = useBlocked(open)
    const openProfile = useProfileCard((state) => state.open)
    const logout = useLogout()
    const profile = me.data

    return (
        <Modal
            open={open}
            onClose={onClose}
            aria-label="Настройки"
            headerStart={<MyStatusButton onEdit={() => onNavigate('status')} />}
        >
            {profile && (
                <div className={styles.head}>
                    <UserAvatar src={profile.avatar_url} shape={profile.avatar_shape} size={96} />
                    <div className={styles.names}>
                        <h2 className={styles.name}>{profile.display_name}</h2>
                        <span className={common.muted}>@{profile.username}</span>
                    </div>
                </div>
            )}

            <div className={common.body}>
                <div className={common.card}>
                    <MenuItem
                        icon="user-circle"
                        onClick={() => {
                            onClose()
                            if (profile) {
                                openProfile(profile.user_id, { remember: false })
                            }
                        }}
                    >
                        Мой профиль
                    </MenuItem>
                </div>

                <div className={common.card}>
                    <MenuItem icon="lock" onClick={() => onNavigate('password')}>
                        Изменить пароль
                    </MenuItem>
                </div>

                <div className={common.card}>
                    <MenuItem icon="shine" onClick={() => onNavigate('interface')}>
                        Параметры интерфейса
                    </MenuItem>
                    <MenuItem icon="protect" onClick={() => onNavigate('privacy')}>
                        Конфиденциальность
                    </MenuItem>
                </div>

                <div className={common.card}>
                    <MenuItem
                        icon="session"
                        onClick={() => onNavigate('sessions')}
                        trailing={
                            sessions.data && <Tag variant="muted">{sessions.data.length}</Tag>
                        }
                    >
                        Активные сеансы
                    </MenuItem>
                    <MenuItem
                        icon="block"
                        onClick={() => onNavigate('blocked')}
                        trailing={
                            blocked.data && blocked.data.length > 0 ? (
                                <Tag variant="muted">
                                    {blocked.data.length}
                                    {blocked.hasNextPage ? '+' : ''}
                                </Tag>
                            ) : undefined
                        }
                    >
                        Чёрный список
                    </MenuItem>
                </div>

                <Button
                    variant="danger"
                    disabled={logout.isPending}
                    onClick={() => logout.mutate()}
                >
                    <Icon name="exit" size={22} color="inherit" />
                    Выйти из аккаунта
                </Button>
            </div>
        </Modal>
    )
}
