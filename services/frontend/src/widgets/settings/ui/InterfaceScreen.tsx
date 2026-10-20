import { useState } from 'react'

import { ACCENTS, useAccent } from '@/features/accent-color'
import { useTheme } from '@/features/toggle-theme'
import { LANGUAGES, useLanguage } from '@/features/ui-language'
import type { Language } from '@/features/ui-language'
import { cn } from '@/shared/lib'
import { Button, ColorPicker, MenuItem, Switch } from '@/shared/ui'

import styles from './InterfaceScreen.module.scss'
import common from './SettingsModal.module.scss'

// настройки интерфейса: язык, тема, акцентный цвет - только для этого устройства
export function InterfaceScreen({ onLanguage }: { onLanguage: () => void }) {
    const { theme, toggle } = useTheme()
    const { accent, setAccent } = useAccent()
    const [pickingAccent, setPickingAccent] = useState(false)

    return (
        <div className={common.body}>
            <div className={common.card}>
                <MenuItem icon="language" onClick={onLanguage}>
                    Язык интерфейса
                </MenuItem>
            </div>

            <div className={common.card}>
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
                    icon="pallete"
                    onClick={() => setPickingAccent((value) => !value)}
                    trailing={
                        <span
                            className={styles.color}
                            style={{ backgroundColor: accent }}
                            aria-hidden="true"
                        />
                    }
                >
                    Акцентный цвет
                </MenuItem>
                {pickingAccent && (
                    <ColorPicker
                        aria-label="Акцентный цвет"
                        palette={ACCENTS}
                        value={accent}
                        onChange={setAccent}
                        className={styles.accents}
                    />
                )}
                <p className={common.hint}>
                    Влияет только на элементы текущего устройства и не заметно другим пользователям.
                </p>
            </div>
        </div>
    )
}

// язык интерфейса; переводы появятся на этапе i18n
export function LanguageScreen({ onSaved }: { onSaved: () => void }) {
    const { language, setLanguage } = useLanguage()
    const [chosen, setChosen] = useState<Language>(language)

    return (
        <div className={common.body}>
            <div role="radiogroup" aria-label="Язык интерфейса" className={common.card}>
                {LANGUAGES.map((option) => (
                    <button
                        key={option.value}
                        type="button"
                        role="radio"
                        aria-checked={option.value === chosen}
                        className={cn(styles.language, option.value === chosen && styles.chosen)}
                        onClick={() => setChosen(option.value)}
                    >
                        <span className={styles.radio} aria-hidden="true" />
                        {option.label}
                    </button>
                ))}
            </div>
            <p className={common.hint}>
                Перевод интерфейса появится позже, пока все тексты на русском.
            </p>

            <Button
                variant="secondary"
                className={common.footer}
                disabled={chosen === language}
                onClick={() => {
                    setLanguage(chosen)
                    onSaved()
                }}
            >
                Сохранить изменения
            </Button>
        </div>
    )
}
