import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { useCustomization } from '@/entities/customization'
import { clearMyAvatarShape, setMyAvatarShape, useMe, UserAvatar } from '@/entities/user'
import type { AvatarShape as Shape, Profile } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { cn, notify } from '@/shared/lib'
import { AvatarShape, Button, ColorPicker, Modal } from '@/shared/ui'

import styles from './AvatarShapeModal.module.scss'

const COLORS = [
    '#D97757',
    '#25E87F',
    '#6CB4F5',
    '#B79CF2',
    '#F5C26B',
    '#FF8787',
    '#F59BC0',
    '#9BE36C',
]

interface Props {
    open: boolean
    onClose: () => void
}

// фигура за моим аватаром: форма из каталога и цвет; видна всем, кроме тех, кого я заблокировал
export function AvatarShapeModal({ open, onClose }: Props) {
    return (
        <Modal open={open} onClose={onClose} title="Фигура">
            {open && <ShapeForm onClose={onClose} />}
        </Modal>
    )
}

function ShapeForm({ onClose }: { onClose: () => void }) {
    const me = useMe()
    const catalog = useCustomization()
    const queryClient = useQueryClient()
    const current = me.data?.avatar_shape ?? null
    const shapes = catalog.data?.avatar_shapes ?? []
    const [shape, setShape] = useState(current?.shape ?? '')
    const [color, setColor] = useState(current?.color ?? COLORS[0]!)
    const chosen = shape || shapes[0] || ''

    const put = (avatar_shape: Shape | null) =>
        queryClient.setQueryData<Profile | null>(['me'], (p) => (p ? { ...p, avatar_shape } : p))
    const onError = (error: unknown) =>
        notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })
    const save = useMutation({
        mutationFn: () => setMyAvatarShape({ shape: chosen, color }),
        onSuccess: (saved) => {
            put(saved)
            onClose()
        },
        onError,
    })
    const remove = useMutation({
        mutationFn: clearMyAvatarShape,
        onSuccess: () => {
            put(null)
            onClose()
        },
        onError,
    })
    const pending = save.isPending || remove.isPending

    return (
        <div className={styles.form}>
            <div className={styles.preview}>
                <UserAvatar
                    src={me.data?.avatar_url}
                    shape={chosen ? { shape: chosen, color } : null}
                    size={96}
                />
            </div>

            <div role="radiogroup" aria-label="Форма" className={styles.grid}>
                {shapes.map((key) => (
                    <button
                        key={key}
                        type="button"
                        role="radio"
                        aria-checked={key === chosen}
                        aria-label={key}
                        title={key}
                        className={cn(styles.option, key === chosen && styles.chosen)}
                        onClick={() => setShape(key)}
                    >
                        <AvatarShape shape={key} color={color}>
                            <span className={styles.dot} />
                        </AvatarShape>
                    </button>
                ))}
            </div>

            <div className={styles.field}>
                <span className={styles.label}>Цвет</span>
                <ColorPicker
                    aria-label="Цвет фигуры"
                    palette={COLORS}
                    value={color}
                    onChange={setColor}
                />
            </div>

            <div className={styles.actions}>
                <Button disabled={!chosen || pending} onClick={() => save.mutate()}>
                    Сохранить
                </Button>
                {current && (
                    <Button variant="danger" disabled={pending} onClick={() => remove.mutate()}>
                        Убрать фигуру
                    </Button>
                )}
            </div>
        </div>
    )
}
