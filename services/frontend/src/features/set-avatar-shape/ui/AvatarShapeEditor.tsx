import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
    clearGroupAvatarShape,
    conversationKey,
    conversationsKey,
    setGroupAvatarShape,
    useConversation,
} from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { useCustomization } from '@/entities/customization'
import { clearMyAvatarShape, setMyAvatarShape, useMe, UserAvatar } from '@/entities/user'
import type { AvatarShape as Shape, Profile } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { cn, notify } from '@/shared/lib'
import { AvatarShape, Button, ColorPicker } from '@/shared/ui'

import styles from './AvatarShapeEditor.module.scss'

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

// чья фигура: моя или группы (меняет только её админ)
export type ShapeTarget = { kind: 'me' } | { kind: 'group'; id: string }

const ME: ShapeTarget = { kind: 'me' }

interface Props {
    // фигура сохранена или убрана - вернуться туда, откуда открыли
    onDone: () => void
    // по умолчанию - моя фигура
    target?: ShapeTarget
}

// фигура за аватаром: форма из каталога и цвет; моя видна всем, кроме тех, кого я заблокировал.
// это содержимое окна, а не отдельное окно: его показывают мой профиль и настройки группы со стрелкой «назад»
export function AvatarShapeEditor({ onDone, target = ME }: Props) {
    const me = useMe()
    const groupId = target.kind === 'group' ? target.id : ''
    const chat = useConversation(groupId, target.kind === 'group')
    const owner = target.kind === 'me' ? me.data : chat.data
    const catalog = useCustomization()
    const queryClient = useQueryClient()
    const current = owner?.avatar_shape ?? null
    const shapes = catalog.data?.avatar_shapes ?? []
    const [shape, setShape] = useState(current?.shape ?? '')
    const [color, setColor] = useState(current?.color ?? COLORS[0]!)
    const chosen = shape || shapes[0] || ''

    const put = (avatar_shape: Shape | null) => {
        if (target.kind === 'me') {
            queryClient.setQueryData<Profile | null>(['me'], (p) =>
                p ? { ...p, avatar_shape } : p,
            )
            return
        }
        queryClient.setQueryData<ConversationSummary>(conversationKey(target.id), (c) =>
            c ? { ...c, avatar_shape } : c,
        )
        void queryClient.invalidateQueries({ queryKey: conversationsKey })
    }
    const onError = (error: unknown) =>
        notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })
    const save = useMutation({
        mutationFn: () =>
            target.kind === 'me'
                ? setMyAvatarShape({ shape: chosen, color })
                : setGroupAvatarShape(target.id, { shape: chosen, color }),
        onSuccess: (saved) => {
            put(saved)
            onDone()
        },
        onError,
    })
    const remove = useMutation({
        mutationFn: () =>
            target.kind === 'me' ? clearMyAvatarShape() : clearGroupAvatarShape(target.id),
        onSuccess: () => {
            put(null)
            onDone()
        },
        onError,
    })
    const pending = save.isPending || remove.isPending

    return (
        <div className={styles.form}>
            <div className={styles.preview}>
                <UserAvatar
                    src={owner?.avatar_url}
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
