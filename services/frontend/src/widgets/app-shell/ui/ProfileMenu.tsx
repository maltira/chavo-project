import { useRef, useState } from 'react'

import { useMe } from '@/entities/user'
import { useLogout } from '@/features/auth-logout'
import { useTheme } from '@/features/toggle-theme'
import { useDismiss } from '@/shared/lib'
import { Avatar, AvatarShape, Icon, IconButton, MenuItem, Status, Switch } from '@/shared/ui'

import styles from './ProfileMenu.module.scss'
import { DEFAULT_WIDTH } from '../model/sidebarWidth'

export function ProfileMenu() {
    const [open, setOpen] = useState(false)
    const [mounted, setMounted] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)
    const me = useMe()
    const { theme, toggle } = useTheme()
    const logout = useLogout()

    const show = () => {
        setMounted(true)
        setOpen(true)
    }
    const hide = () => setOpen(false)

    useDismiss(open, rootRef, hide)

    const profile = me.data

    return (
        <div ref={rootRef}>
            <IconButton
                icon="menu-lines"
                aria-label="Меню"
                aria-haspopup="menu"
                aria-expanded={open}
                isActive={open}
                onClick={open ? hide : show}
            />

            {mounted && (
                <div
                    role="menu"
                    aria-label="Меню профиля"
                    className={styles.menu}
                    data-state={open ? 'open' : 'closed'}
                    style={{ minWidth: DEFAULT_WIDTH + 'px' }}
                    onAnimationEnd={() => {
                        if (!open) {
                            setMounted(false)
                        }
                    }}
                >
                    {profile && (
                        <div className={styles.user}>
                            <AvatarShape
                                shape={profile.avatar_shape?.shape}
                                color={profile.avatar_shape?.color}
                            >
                                <Avatar src={profile.avatar_url} size={42} alt="" />
                            </AvatarShape>
                            <div className={styles.names}>
                                <span className={styles.name}>{profile.display_name}</span>
                                <span className={styles.username}>@{profile.username}</span>
                            </div>
                            <Icon name="arrow-right" size={24} className={styles.arrow} />
                        </div>
                    )}

                    {profile?.status && (
                        <Status
                            icon={profile.status.icon}
                            text={profile.status.text}
                            background={profile.status.background}
                            foreground={profile.status.foreground}
                            expiresAt={profile.status.expires_at}
                        />
                    )}

                    <div className={styles.items}>
                        <MenuItem
                            icon={theme === 'dark' ? 'theme-dark' : 'theme-light'}
                            trailing={
                                <Switch
                                    aria-label="Тёмная тема"
                                    checked={theme === 'dark'}
                                    onChange={toggle}
                                />
                            }
                        >
                            Тёмная тема
                        </MenuItem>
                        <MenuItem icon="settings" onClick={hide}>
                            Настройки
                        </MenuItem>
                        <MenuItem icon="help" onClick={hide}>
                            FAQ и поддержка
                        </MenuItem>
                        <MenuItem
                            icon="exit"
                            danger
                            disabled={logout.isPending}
                            onClick={() => logout.mutate()}
                        >
                            Выйти из аккаунта
                        </MenuItem>
                    </div>
                </div>
            )}
        </div>
    )
}
