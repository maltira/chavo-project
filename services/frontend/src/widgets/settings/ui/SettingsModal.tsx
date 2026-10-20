import { useState } from 'react'

import { StatusEditor } from '@/features/set-status'
import { Icon, Modal } from '@/shared/ui'

import { BlockedScreen } from './BlockedScreen'
import { InterfaceScreen, LanguageScreen } from './InterfaceScreen'
import { MainScreen } from './MainScreen'
import { PasswordScreen } from './PasswordScreen'
import { PrivacyScreen } from './PrivacyScreen'
import { SessionsScreen } from './SessionsScreen'
import styles from './SettingsModal.module.scss'

export type Screen =
    'main' | 'status' | 'password' | 'interface' | 'language' | 'privacy' | 'sessions' | 'blocked'

const TITLES: Record<Exclude<Screen, 'main'>, string> = {
    status: 'Статус',
    password: 'Изменить пароль',
    interface: 'Настройки интерфейса',
    language: 'Язык интерфейса',
    privacy: 'Конфиденциальность',
    sessions: 'Активные сеансы',
    blocked: 'Чёрный список',
}

// «назад» ведёт на уровень выше: язык - в настройки интерфейса, остальное - на главный экран
const PARENT: Record<Exclude<Screen, 'main'>, Screen> = {
    status: 'main',
    password: 'main',
    interface: 'main',
    language: 'interface',
    privacy: 'main',
    sessions: 'main',
    blocked: 'main',
}

interface Props {
    open: boolean
    onClose: () => void
}

// настройки: главный экран со своим профилем и разделы внутри одного окна
export function SettingsModal({ open, onClose }: Props) {
    // при каждом открытии - с главного экрана
    const [screen, setScreen] = useState<Screen>('main')
    const [wasOpen, setWasOpen] = useState(open)
    if (open !== wasOpen) {
        setWasOpen(open)
        if (open) {
            setScreen('main')
        }
    }

    if (screen === 'main') {
        return <MainScreen open={open} onClose={onClose} onNavigate={setScreen} />
    }

    return (
        <Modal
            open={open}
            onClose={onClose}
            title={TITLES[screen]}
            headerStart={
                <button
                    type="button"
                    className={styles.back}
                    aria-label="Назад"
                    onClick={() => setScreen(PARENT[screen])}
                >
                    <Icon name="arrow-left" size={24} color="inherit" />
                </button>
            }
            className={styles.screen}
        >
            {screen === 'status' && <StatusEditor onDone={() => setScreen('main')} />}
            {screen === 'password' && <PasswordScreen />}
            {screen === 'interface' && <InterfaceScreen onLanguage={() => setScreen('language')} />}
            {screen === 'language' && <LanguageScreen onSaved={() => setScreen('interface')} />}
            {screen === 'privacy' && <PrivacyScreen onSaved={() => setScreen('main')} />}
            {screen === 'sessions' && <SessionsScreen />}
            {screen === 'blocked' && <BlockedScreen />}
        </Modal>
    )
}
