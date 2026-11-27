import { useState } from 'react'

import { StatusEditor } from '@/features/set-status'
import { Icon, Modal } from '@/shared/ui'

import { BlockedScreen } from './BlockedScreen'
import { DeleteAccountScreen } from './DeleteAccountScreen'
import { InterfaceScreen, LanguageScreen } from './InterfaceScreen'
import { MainScreen } from './MainScreen'
import { PasswordScreen } from './PasswordScreen'
import { PrivacyScreen } from './PrivacyScreen'
import { SessionsScreen } from './SessionsScreen'
import styles from './SettingsModal.module.scss'
import { TwoFactorScreen } from './TwoFactorScreen'

export type Screen =
    | 'main'
    | 'status'
    | 'password'
    | 'two-factor'
    | 'delete-account'
    | 'interface'
    | 'language'
    | 'privacy'
    | 'sessions'
    | 'blocked'

const TITLES: Record<Exclude<Screen, 'main'>, string> = {
    status: 'Статус',
    password: 'Изменить пароль',
    'two-factor': 'Двухфакторная защита',
    'delete-account': 'Удаление аккаунта',
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
    'two-factor': 'main',
    'delete-account': 'main',
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
    const [twoFactorCompact, setTwoFactorCompact] = useState(false)
    const [wasOpen, setWasOpen] = useState(open)
    if (open !== wasOpen) {
        setWasOpen(open)
        if (open) {
            setScreen('main')
            setTwoFactorCompact(false)
        }
    }

    if (screen === 'main') {
        return <MainScreen open={open} onClose={onClose} onNavigate={setScreen} />
    }

    if (screen === 'delete-account') {
        return (
            <DeleteAccountScreen
                open={open}
                onClose={onClose}
                onBack={() => setScreen('main')}
            />
        )
    }

    const compact = screen === 'two-factor' && twoFactorCompact

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
                    onClick={() => {
                        setTwoFactorCompact(false)
                        setScreen(PARENT[screen])
                    }}
                >
                    <Icon name="arrow-left" size={24} color="inherit" />
                </button>
            }
            className={compact ? undefined : styles.screen}
        >
            {screen === 'status' && <StatusEditor onDone={() => setScreen('main')} />}
            {screen === 'password' && <PasswordScreen />}
            {screen === 'two-factor' && <TwoFactorScreen onCompactChange={setTwoFactorCompact} />}
            {screen === 'interface' && <InterfaceScreen onLanguage={() => setScreen('language')} />}
            {screen === 'language' && <LanguageScreen onSaved={() => setScreen('interface')} />}
            {screen === 'privacy' && <PrivacyScreen onSaved={() => setScreen('main')} />}
            {screen === 'sessions' && <SessionsScreen />}
            {screen === 'blocked' && <BlockedScreen />}
        </Modal>
    )
}
