import { useState } from 'react'

import { slugSchema, SlugField, useDirectPeers } from '@/entities/conversation'
import { UserRow } from '@/entities/user'
import { ApiError, errorMessage } from '@/shared/api'
import { usePhotoDraft } from '@/shared/lib'
import {
    Avatar,
    Button,
    Field,
    Icon,
    IconButton,
    Input,
    Modal,
    PhotoPicker,
    Segmented,
    Textarea,
    TextLink,
} from '@/shared/ui'

import { useCreateGroup } from '../model/useCreateGroup'
import styles from './CreateGroupModal.module.scss'

type Visibility = 'public' | 'private'

const VISIBILITY_HINT: Record<Visibility, string> = {
    public: 'К открытому чату может присоединиться любой пользователь без приглашения и одобрения.',
    private:
        'В закрытый чат можно попасть только по ссылке-приглашению и с одобрения администратора.',
}

// пределы сервера
const NAME_MAX = 100
const DESCRIPTION_MAX = 255

interface Props {
    open: boolean
    onClose: () => void
}

// групповой чат в два шага: название, тип и описание, затем участники из моих собеседников
export function CreateGroupModal({ open, onClose }: Props) {
    // форма живёт, пока окно открыто: при следующем открытии всё с чистого листа
    const [session, setSession] = useState(0)
    const [wasOpen, setWasOpen] = useState(open)
    if (open !== wasOpen) {
        setWasOpen(open)
        if (open) {
            setSession((n) => n + 1)
        }
    }
    return <CreateGroupForm key={session} open={open} onClose={onClose} />
}

function CreateGroupForm({ open, onClose }: Props) {
    const [step, setStep] = useState<'details' | 'members'>('details')
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [visibility, setVisibility] = useState<Visibility>('public')
    const [slug, setSlug] = useState('')
    const [slugError, setSlugError] = useState<string>()
    const [selected, setSelected] = useState<string[]>([])
    const create = useCreateGroup(onClose)
    // фото ждёт создания группы; при следующем открытии окна форма с чистого листа
    const draft = usePhotoDraft()

    // кого можно позвать: собеседники из моих личных чатов
    const peers = useDirectPeers()

    const toggle = (userId: string) =>
        setSelected((ids) =>
            ids.includes(userId) ? ids.filter((id) => id !== userId) : [...ids, userId],
        )

    function submit() {
        create.mutate(
            {
                input: {
                    name: name.trim(),
                    description: description.trim() || undefined,
                    visibility,
                    slug: visibility === 'public' ? slug.trim() : undefined,
                    member_ids: selected,
                },
                photo: draft.photo,
            },
            {
                onError: (error) => {
                    // ссылка занята - вернуться к полю ссылки
                    if (error instanceof ApiError && error.reason?.startsWith('SLUG_')) {
                        setSlugError(errorMessage(error))
                        setStep('details')
                    }
                },
            },
        )
    }

    function next() {
        if (!name.trim()) {
            return
        }
        if (visibility === 'public') {
            const checked = slugSchema.safeParse(slug)
            if (!checked.success) {
                setSlugError(checked.error.issues[0]?.message)
                return
            }
        }
        setStep('members')
    }

    if (step === 'members') {
        return (
            <Modal
                open={open}
                onClose={onClose}
                title="Участники"
                headerStart={
                    <button
                        type="button"
                        className={styles.back}
                        aria-label="Назад"
                        onClick={() => setStep('details')}
                    >
                        <Icon name="arrow-left" size={24} color="inherit" />
                    </button>
                }
                className={styles.tall}
            >
                <div className={styles.members}>
                    <p className={styles.label}>Выберите участников (опционально):</p>
                    {peers.length > 0 ? (
                        <div className={styles.list}>
                            {peers.map((userId) => {
                                const chosen = selected.includes(userId)
                                return (
                                    <UserRow
                                        key={userId}
                                        userId={userId}
                                        aside={(profile) => (
                                            <IconButton
                                                icon={chosen ? 'check' : 'add'}
                                                size={40}
                                                aria-label={`${chosen ? 'Убрать' : 'Добавить'} ${profile?.display_name ?? ''}`.trim()}
                                                aria-pressed={chosen}
                                                className={styles.toggle}
                                                onClick={() => toggle(userId)}
                                            />
                                        )}
                                    />
                                )
                            })}
                        </div>
                    ) : (
                        <p className={styles.hint}>
                            Здесь появятся люди, с которыми вы переписываетесь
                        </p>
                    )}
                </div>

                <div className={styles.footer}>
                    <p className={styles.label}>Выбрано участников: {selected.length}</p>
                    <Button disabled={create.isPending} onClick={submit}>
                        Создать чат
                    </Button>
                </div>
            </Modal>
        )
    }

    return (
        <Modal open={open} onClose={onClose} aria-label="Новый групповой чат">
            {/* фото уйдёт на сервер сразу после создания группы */}
            <div className={styles.photo}>
                <Avatar src={draft.preview} size={96} />
                <div className={styles.photoActions}>
                    <PhotoPicker onPick={draft.set}>
                        {(open) => (
                            <TextLink as="button" className={styles.photoButton} onClick={open}>
                                {draft.photo ? 'Сменить фото' : 'Выбрать фото'}
                            </TextLink>
                        )}
                    </PhotoPicker>
                    {draft.photo && (
                        <TextLink as="button" className={styles.photoRemove} onClick={draft.clear}>
                            Убрать
                        </TextLink>
                    )}
                </div>
            </div>

            <form
                className={styles.fields}
                onSubmit={(event) => {
                    event.preventDefault()
                    next()
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
                            autoFocus
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
                    <p className={styles.hint}>{VISIBILITY_HINT[visibility]}</p>
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

                <Button type="submit" disabled={!name.trim()}>
                    Продолжить
                </Button>
            </form>
        </Modal>
    )
}
