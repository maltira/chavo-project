import { useState } from 'react'

import { durationLabel, useCustomization } from '@/entities/customization'
import { cn } from '@/shared/lib'
import { Button, ColorPicker, Icon, Input, Segmented, Status } from '@/shared/ui'

import { ME } from '../model/target'
import type { CustomTarget } from '../model/target'
import { useStatusMutations, useTargetStatus } from '../model/useTargetStatus'
import styles from './StatusEditor.module.scss'

// готовые цвета: фон и общий цвет текста и иконки; любой другой - через пипетку
const BACKGROUNDS = [
    '#D97757',
    '#25E87F',
    '#6CB4F5',
    '#B79CF2',
    '#F5C26B',
    '#FF8787',
    '#FAF9F5',
    '#2A2A2A',
]
const FOREGROUNDS = ['#141413', '#FAF9F5', '#512315', '#064E3B', '#1E3A8A', '#7A1F3D']

// предел сервера
const TEXT_MAX = 70

interface Props {
    // статус сохранён или снят - вернуться туда, откуда открыли
    onDone: () => void
    // по умолчанию - мой статус
    target?: CustomTarget
}

// установка статуса: иконка, текст, цвета и срок; статус всегда ставится целиком заново.
// это содержимое окна, а не отдельное окно: его показывают настройки, мой профиль и настройки группы
export function StatusEditor({ onDone, target = ME }: Props) {
    const catalog = useCustomization()
    const current = useTargetStatus(target)
    const icons = catalog.data?.status.icons ?? []
    const durations = catalog.data?.status.durations ?? []
    const { set, clear, pending } = useStatusMutations(target, onDone)

    // форма начинается с текущего статуса, если он есть
    const [icon, setIcon] = useState(current?.icon ?? '')
    const [text, setText] = useState(current?.text ?? '')
    const [background, setBackground] = useState(current?.background ?? BACKGROUNDS[0]!)
    const [foreground, setForeground] = useState(current?.foreground ?? FOREGROUNDS[0]!)
    const [duration, setDuration] = useState('24h')

    const chosenIcon = icon || icons[0] || ''
    const chosenDuration = durations.includes(duration) ? duration : (durations[0] ?? '')
    const canSave = text.trim().length > 0 && !!chosenIcon && !!chosenDuration && !pending

    return (
        <div className={styles.form}>
            <div className={styles.preview}>
                <Status
                    icon={chosenIcon}
                    text={text.trim() || (target.kind === 'me' ? 'Ваш статус' : 'Статус группы')}
                    background={background}
                    foreground={foreground}
                />
            </div>

            <div className={styles.field}>
                <span className={styles.label}>Иконка</span>
                <div role="radiogroup" aria-label="Иконка" className={styles.icons}>
                    {icons.map((key) => (
                        <button
                            key={key}
                            type="button"
                            role="radio"
                            aria-checked={key === chosenIcon}
                            aria-label={key}
                            className={cn(styles.icon, key === chosenIcon && styles.chosen)}
                            onClick={() => setIcon(key)}
                        >
                            <Icon name={`shape-${key}`} size={24} color="inherit" />
                        </button>
                    ))}
                </div>
            </div>

            <label className={styles.field}>
                <span className={styles.label}>Текст</span>
                <Input
                    placeholder="Например, «Сегодня отдыхаю»"
                    maxLength={TEXT_MAX}
                    value={text}
                    // перенос строки сервер не примет
                    onChange={(event) => setText(event.target.value.replace(/[\r\n]/g, ' '))}
                    autoFocus
                />
            </label>

            <div className={styles.field}>
                <span className={styles.label}>Фон</span>
                <ColorPicker
                    aria-label="Фон"
                    palette={BACKGROUNDS}
                    value={background}
                    onChange={setBackground}
                />
            </div>

            <div className={styles.field}>
                <span className={styles.label}>Цвет текста и иконки</span>
                <ColorPicker
                    aria-label="Цвет текста и иконки"
                    palette={FOREGROUNDS}
                    value={foreground}
                    onChange={setForeground}
                />
            </div>

            {durations.length > 0 && (
                <div className={styles.field}>
                    <span className={styles.label}>Сколько показывать</span>
                    <Segmented
                        aria-label="Срок статуса"
                        value={chosenDuration}
                        onChange={setDuration}
                        options={durations.map((key) => ({
                            value: key,
                            label: durationLabel(key),
                        }))}
                    />
                </div>
            )}

            <div className={styles.actions}>
                <Button
                    disabled={!canSave}
                    onClick={() =>
                        set.mutate({
                            icon: chosenIcon,
                            text: text.trim(),
                            background,
                            foreground,
                            duration: chosenDuration,
                        })
                    }
                >
                    Сохранить
                </Button>
                {current && (
                    <Button variant="danger" disabled={pending} onClick={() => clear.mutate()}>
                        Снять статус
                    </Button>
                )}
            </div>
        </div>
    )
}
