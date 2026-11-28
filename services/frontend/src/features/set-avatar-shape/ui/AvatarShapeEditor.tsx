import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
    clearGroupAvatarShape,
    clearGroupProfileCover,
    conversationKey,
    conversationsKey,
    setGroupAvatarShape,
    setGroupProfileCover,
    useConversation,
} from '@/entities/conversation'
import type { ConversationSummary } from '@/entities/conversation'
import { useCustomization } from '@/entities/customization'
import {
    clearMyAvatarShape,
    clearMyProfileCover,
    setMyAvatarShape,
    setMyProfileCover,
    useMe,
    UserAvatar,
} from '@/entities/user'
import type { AvatarShape as Shape, Profile, ProfileCover as Cover } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { cn, notify, plural } from '@/shared/lib'
import {
    AvatarShape,
    Button,
    ColorPicker,
    CoverPatternIcon,
    ProfileCover,
    Segmented,
} from '@/shared/ui'

import styles from './AvatarShapeEditor.module.scss'

const SHAPE_COLORS = [
    '#D97757',
    '#25E87F',
    '#6CB4F5',
    '#B79CF2',
    '#F5C26B',
    '#FF8787',
    '#F59BC0',
    '#9BE36C',
]

const COVER_COLORS = [
    '#4F46E5',
    '#0EA5E9',
    '#10B981',
    '#F59E0B',
    '#EF4444',
    '#EC4899',
    '#8B5CF6',
    '#D97757',
]

const PATTERN_LABELS: Record<string, string> = {
    star: 'Звёзды',
    heart: 'Сердца',
    shine: 'Искры',
    moon: 'Луна',
    flame: 'Огонь',
    clover: 'Клевер',
    crown: 'Корона',
    bolt: 'Молнии',
    flower: 'Цветы',
    music: 'Ноты',
    paw: 'Лапки',
    diamond: 'Кристаллы',
}

type Tab = 'shape' | 'cover'

// чьё оформление: моё или группы (меняет только её админ)
export type ShapeTarget = { kind: 'me' } | { kind: 'group'; id: string }

const ME: ShapeTarget = { kind: 'me' }

interface Props {
    // оформление сохранено или убрано - вернуться туда, откуда открыли
    onDone: () => void
    // по умолчанию - моё оформление
    target?: ShapeTarget
}

// единый редактор «Оформление»: живое превью с обложкой и фигурой + вкладки «Фигура» и «Обложка»
export function AvatarShapeEditor({ onDone, target = ME }: Props) {
    const me = useMe()
    const groupId = target.kind === 'group' ? target.id : ''
    const chat = useConversation(groupId, target.kind === 'group')
    const owner = target.kind === 'me' ? me.data : chat.data
    const catalog = useCustomization()
    const queryClient = useQueryClient()

    const currentShape = owner?.avatar_shape ?? null
    const currentCover = owner?.profile_cover ?? null
    const shapes = catalog.data?.avatar_shapes ?? []
    const patterns = catalog.data?.profile_cover_patterns ?? []

    const [tab, setTab] = useState<Tab>('shape')

    // черновик фигуры
    const [shape, setShape] = useState(currentShape?.shape ?? '')
    const [shapeColor, setShapeColor] = useState(currentShape?.color ?? SHAPE_COLORS[0]!)
    const [shapeTouched, setShapeTouched] = useState(false)
    const chosenShape = shape || shapes[0] || ''

    // черновик обложки
    const [coverColor, setCoverColor] = useState(currentCover?.color ?? COVER_COLORS[0]!)
    const [coverPattern, setCoverPattern] = useState(currentCover?.pattern ?? '')
    const [coverTouched, setCoverTouched] = useState(false)

    const previewShape: Shape | null =
        (tab === 'shape' || shapeTouched) && chosenShape
            ? { shape: chosenShape, color: shapeColor }
            : currentShape
    const previewCover: Cover | null =
        tab === 'cover' || coverTouched
            ? { color: coverColor, pattern: coverPattern }
            : currentCover

    const patchCache = (patch: { avatar_shape?: Shape | null; profile_cover?: Cover | null }) => {
        if (target.kind === 'me') {
            queryClient.setQueryData<Profile | null>(['me'], (p) => (p ? { ...p, ...patch } : p))
            return
        }
        queryClient.setQueryData<ConversationSummary>(conversationKey(target.id), (c) =>
            c ? { ...c, ...patch } : c,
        )
        void queryClient.invalidateQueries({ queryKey: conversationsKey })
    }

    const onError = (error: unknown) =>
        notify({ icon: 'warn', title: 'Не получилось', description: errorMessage(error) })

    const saveShape = (next: Shape) =>
        target.kind === 'me' ? setMyAvatarShape(next) : setGroupAvatarShape(target.id, next)
    const saveCover = (next: Cover) =>
        target.kind === 'me' ? setMyProfileCover(next) : setGroupProfileCover(target.id, next)

    const wantSaveShape = (tab === 'shape' || shapeTouched) && !!chosenShape
    const wantSaveCover = tab === 'cover' || coverTouched

    const save = useMutation({
        mutationFn: async () => {
            const [savedShape, savedCover] = await Promise.all([
                wantSaveShape ? saveShape({ shape: chosenShape, color: shapeColor }) : null,
                wantSaveCover ? saveCover({ color: coverColor, pattern: coverPattern }) : null,
            ])
            return { savedShape, savedCover }
        },
        onSuccess: ({ savedShape, savedCover }) => {
            const patch: { avatar_shape?: Shape | null; profile_cover?: Cover | null } = {}
            if (savedShape) {
                patch.avatar_shape = savedShape
            }
            if (savedCover) {
                patch.profile_cover = savedCover
            }
            patchCache(patch)
            notify({ icon: 'check', title: 'Оформление сохранено' })
            onDone()
        },
        onError,
    })

    const removeShape = useMutation({
        mutationFn: () =>
            target.kind === 'me' ? clearMyAvatarShape() : clearGroupAvatarShape(target.id),
        onSuccess: () => {
            patchCache({ avatar_shape: null })
            onDone()
        },
        onError,
    })

    const removeCover = useMutation({
        mutationFn: () =>
            target.kind === 'me' ? clearMyProfileCover() : clearGroupProfileCover(target.id),
        onSuccess: () => {
            patchCache({ profile_cover: null })
            onDone()
        },
        onError,
    })

    const pending = save.isPending || removeShape.isPending || removeCover.isPending
    const canSave = (wantSaveShape || wantSaveCover) && !pending

    const previewName =
        target.kind === 'me' ? (me.data?.display_name ?? '') : (chat.data?.name ?? 'Группа')
    const previewSub =
        target.kind === 'me'
            ? me.data?.username
                ? `@${me.data.username}`
                : ''
            : chat.data
              ? `${chat.data.members_count} ${plural(chat.data.members_count, ['участник', 'участника', 'участников'])}`
              : ''

    return (
        <div className={styles.form}>
            <div className={styles.previewCard}>
                {previewCover && (
                    <ProfileCover
                        color={previewCover.color}
                        pattern={previewCover.pattern}
                        compact
                    />
                )}
                <div className={styles.previewContent}>
                    <UserAvatar src={owner?.avatar_url} shape={previewShape} size={84} />
                    {previewName && (
                        <div className={styles.previewNames}>
                            <span className={styles.previewName}>{previewName}</span>
                            {previewSub && <span className={styles.previewSub}>{previewSub}</span>}
                        </div>
                    )}
                </div>
            </div>

            <Segmented
                aria-label="Раздел оформления"
                value={tab}
                onChange={setTab}
                options={[
                    { value: 'shape', label: 'Фигура' },
                    { value: 'cover', label: 'Обложка' },
                ]}
            />

            {tab === 'shape' ? (
                <>
                    <div role="radiogroup" aria-label="Форма" className={styles.grid}>
                        {shapes.map((key) => (
                            <button
                                key={key}
                                type="button"
                                role="radio"
                                aria-checked={key === chosenShape}
                                aria-label={key}
                                title={key}
                                className={cn(styles.option, key === chosenShape && styles.chosen)}
                                onClick={() => {
                                    setShape(key)
                                    setShapeTouched(true)
                                }}
                            >
                                <AvatarShape shape={key} color={shapeColor}>
                                    <span className={styles.dot} />
                                </AvatarShape>
                            </button>
                        ))}
                    </div>

                    <div className={styles.field}>
                        <span className={styles.label}>Цвет фигуры</span>
                        <ColorPicker
                            aria-label="Цвет фигуры"
                            palette={SHAPE_COLORS}
                            value={shapeColor}
                            onChange={(next) => {
                                setShapeColor(next)
                                setShapeTouched(true)
                            }}
                        />
                    </div>
                </>
            ) : (
                <>
                    <div className={styles.field}>
                        <span className={styles.label}>Цвет обложки</span>
                        <ColorPicker
                            aria-label="Цвет обложки"
                            palette={COVER_COLORS}
                            value={coverColor}
                            onChange={(next) => {
                                setCoverColor(next)
                                setCoverTouched(true)
                            }}
                        />
                    </div>

                    <div className={styles.field}>
                        <span className={styles.label}>Паттерн</span>
                        <div
                            role="radiogroup"
                            aria-label="Паттерн обложки"
                            className={styles.patternGrid}
                        >
                            <button
                                type="button"
                                role="radio"
                                aria-checked={coverPattern === ''}
                                aria-label="Без паттерна"
                                title="Без паттерна"
                                className={cn(styles.option, coverPattern === '' && styles.chosen)}
                                onClick={() => {
                                    setCoverPattern('')
                                    setCoverTouched(true)
                                }}
                            >
                                <svg
                                    viewBox="0 0 24 24"
                                    width={20}
                                    height={20}
                                    fill="none"
                                    stroke="currentColor"
                                    strokeWidth="1.8"
                                    strokeLinecap="round"
                                    aria-hidden="true"
                                    focusable="false"
                                >
                                    <circle cx="12" cy="12" r="8" />
                                    <line x1="6.3" y1="17.7" x2="17.7" y2="6.3" />
                                </svg>
                            </button>
                            {patterns.map((key) => {
                                const label = PATTERN_LABELS[key] ?? key
                                return (
                                    <button
                                        key={key}
                                        type="button"
                                        role="radio"
                                        aria-checked={coverPattern === key}
                                        aria-label={label}
                                        title={label}
                                        className={cn(
                                            styles.option,
                                            coverPattern === key && styles.chosen,
                                        )}
                                        style={{ color: coverColor }}
                                        onClick={() => {
                                            setCoverPattern(key)
                                            setCoverTouched(true)
                                        }}
                                    >
                                        <CoverPatternIcon pattern={key} size={22} />
                                    </button>
                                )
                            })}
                        </div>
                    </div>
                </>
            )}

            <div className={styles.actions}>
                <Button disabled={!canSave} onClick={() => save.mutate()}>
                    Сохранить
                </Button>
                {tab === 'shape' && currentShape && (
                    <Button
                        variant="danger"
                        disabled={pending}
                        onClick={() => removeShape.mutate()}
                    >
                        Убрать фигуру
                    </Button>
                )}
                {tab === 'cover' && currentCover && (
                    <Button
                        variant="danger"
                        disabled={pending}
                        onClick={() => removeCover.mutate()}
                    >
                        Убрать обложку
                    </Button>
                )}
            </div>
        </div>
    )
}
