import { useRef, useState } from 'react'

import { useMe, useProfileCard } from '@/entities/user'
import { useLogout } from '@/features/auth/logout'
import { useTheme } from '@/features/toggle-theme'
import { ROUTES } from '@/shared/config'
import { useDismiss } from '@/shared/lib'
import { StatusButton, StatusEditor } from '@/features/set-status'
import { Avatar, AvatarShape, Icon, IconButton, MenuItem, Modal, Switch } from '@/shared/ui'

import styles from './ProfileMenu.module.scss'
import { DEFAULT_WIDTH } from '../model/sidebarWidth'

interface Props {
    // «Настройки» - окно настроек показывает страница
    onSettings?: () => void
}

export function ProfileMenu({ onSettings }: Props) {
    const [open, setOpen] = useState(false)
    const [mounted, setMounted] = useState(false)
    const [editingStatus, setEditingStatus] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)
    const me = useMe()
    const openProfile = useProfileCard((state) => state.open)
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
                        <button
                            type="button"
                            role="menuitem"
                            className={styles.user}
                            onClick={() => {
                                hide()
                                // свой профиль в «Недавние» не попадает
                                openProfile(profile.user_id, { remember: false })
                            }}
                        >
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
                        </button>
                    )}

                    {/* статус нажимается: меню закрывается, открывается редактор статуса */}
                    {profile && (
                        <StatusButton
                            block
                            onEdit={() => {
                                hide()
                                setEditingStatus(true)
                            }}
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
                        <MenuItem
                            icon="settings"
                            onClick={() => {
                                hide()
                                onSettings?.()
                            }}
                        >
                            Настройки
                        </MenuItem>
                        <MenuItem
                            icon="help"
                            onClick={() => {
                                hide()
                                // в новой вкладке: открытый чат и соединение остаются
                                window.open(ROUTES.faq, '_blank', 'noopener,noreferrer')
                            }}
                        >
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

            {/* редактор статуса из меню - отдельное окно, меню к этому моменту закрыто */}
            <Modal open={editingStatus} onClose={() => setEditingStatus(false)} title="Статус">
                {editingStatus && <StatusEditor onDone={() => setEditingStatus(false)} />}
            </Modal>
        </div>
    )
}
