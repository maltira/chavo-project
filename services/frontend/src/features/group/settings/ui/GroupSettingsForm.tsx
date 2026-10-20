import { useState } from 'react'

import { slugSchema, SlugField } from '@/entities/conversation'
import type { ConversationSummary, GroupUpdate } from '@/entities/conversation'
import { UserAvatar } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { Button, ConfirmModal, Field, Input, Segmented, Textarea, TextLink } from '@/shared/ui'

import { isSlugError, useDeleteGroup, useUpdateGroup } from '../model/useGroupSettings'
import styles from './GroupSettingsForm.module.scss'

type Visibility = ConversationSummary['visibility']

const VISIBILITY_HINT: Record<Visibility, string> = {
    public: 'К открытому чату может присоединиться любой пользователь без приглашения и одобрения.',
    private:
        'В закрытый чат можно попасть только по ссылке-приглашению и с одобрения администратора.',
}

// пределы сервера
const NAME_MAX = 100
const DESCRIPTION_MAX = 255

interface Props {
    chat: ConversationSummary
    // клик по аватару - выбор фигуры за ним
    onPickShape: () => void
    // сохранено - назад к сведениям
    onSaved: () => void
    // группа удалена - окно закрывается
    onDeleted: () => void
}

// настройки группы (только админ): название, тип, ссылка открытой группы, описание, удаление
export function GroupSettingsForm({ chat, onPickShape, onSaved, onDeleted }: Props) {
    // база сравнения - значения на момент открытия: отправляются только изменённые поля
    const [initial] = useState(chat)
    const [name, setName] = useState(initial.name ?? '')
    const [visibility, setVisibility] = useState<Visibility>(initial.visibility)
    const [slug, setSlug] = useState(initial.slug ?? '')
    const [description, setDescription] = useState(initial.description ?? '')
    const [slugError, setSlugError] = useState<string>()
    const [confirmDelete, setConfirmDelete] = useState(false)

    const update = useUpdateGroup(chat.id, onSaved)
    const remove = useDeleteGroup(chat.id, onDeleted)

    function patch(): GroupUpdate | null {
        const next: GroupUpdate = {}
        if (name.trim() !== (initial.name ?? '')) {
            next.name = name.trim()
        }
        if (description.trim() !== (initial.description ?? '')) {
            next.description = description.trim()
        }
        if (visibility !== initial.visibility) {
            next.visibility = visibility
        }
        if (visibility === 'public' && slug.trim() !== (initial.slug ?? '')) {
            next.slug = slug.trim()
        }
        return Object.keys(next).length > 0 ? next : null
    }

    const changes = patch()
    const canSave = !!changes && !!name.trim() && !update.isPending

    function submit() {
        if (!changes) {
            return
        }
        if (visibility === 'public') {
            const checked = slugSchema.safeParse(slug)
            if (!checked.success) {
                setSlugError(checked.error.issues[0]?.message)
                return
            }
        }
        update.mutate(changes, {
            onError: (error) => {
                if (isSlugError(error)) {
                    setSlugError(errorMessage(error))
                }
            },
        })
    }

    return (
        <>
            <div className={styles.photo}>
                <button
                    type="button"
                    className={styles.avatar}
                    aria-label="Изменить фигуру за аватаром"
                    onClick={onPickShape}
                >
                    <UserAvatar src={chat.avatar_url} shape={chat.avatar_shape} size={96} />
                </button>
                <TextLink as="button" className={styles.photoButton} disabled>
                    Выбрать фото
                </TextLink>
            </div>

            <form
                className={styles.fields}
                onSubmit={(event) => {
                    event.preventDefault()
                    submit()
                }}
            >
                <Field label="Название">
                    {(props) => (
                        <Input
                            {...props}
                            placeholder="Название чата"
                            maxLength={NAME_MAX}
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                        />
                    )}
                </Field>

                <div className={styles.field}>
                    <span className={styles.label}>Тип чата</span>
                    <Segmented
                        aria-label="Тип чата"
                        value={visibility}
                        onChange={setVisibility}
                        options={[
                            { value: 'public', label: 'Открытый' },
                            { value: 'private', label: 'Закрытый' },
                        ]}
                    />
                    <p className={styles.hint}>
                        {VISIBILITY_HINT[visibility]}
                        {visibility !== initial.visibility &&
                            ' Прежняя ссылка на чат перестанет работать.'}
                    </p>
                </div>

                {visibility === 'public' && (
                    <SlugField
                        value={slug}
                        error={slugError}
                        onChange={(value) => {
                            setSlug(value)
                            setSlugError(undefined)
                        }}
                    />
                )}

                <Field label="Описание">
                    {(props) => (
                        <Textarea
                            {...props}
                            className={styles.description}
                            placeholder="По желанию укажите на какую тему будет этот групповой чат"
                            maxLength={DESCRIPTION_MAX}
                            value={description}
                            onChange={(event) => setDescription(event.target.value)}
                        />
                    )}
                </Field>

                <div className={styles.actions}>
                    <Button type="submit" variant="secondary" disabled={!canSave}>
                        Сохранить изменения
                    </Button>
                    <Button
                        variant="danger"
                        icon="trash"
                        disabled={remove.isPending}
                        onClick={() => setConfirmDelete(true)}
                    >
                        Удалить чат
                    </Button>
                </div>
            </form>

            <ConfirmModal
                open={confirmDelete}
                onClose={() => setConfirmDelete(false)}
                onConfirm={() => {
                    setConfirmDelete(false)
                    remove.mutate()
                }}
                confirmLabel="Удалить"
                danger
                pending={remove.isPending}
            >
                Группа и вся её история будут удалены у всех участников
            </ConfirmModal>
        </>
    )
}
