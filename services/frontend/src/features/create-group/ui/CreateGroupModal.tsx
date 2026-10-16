import { useState } from 'react'

import { useConversations } from '@/entities/conversation'
import { UserAvatar, useUser } from '@/entities/user'
import {
    Avatar,
    Button,
    Field,
    Icon,
    IconButton,
    Input,
    Modal,
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
    const [selected, setSelected] = useState<string[]>([])
    const create = useCreateGroup(onClose)

    // кого можно позвать: собеседники из моих личных чатов
    const chats = useConversations()
    const peers = (chats.data ?? [])
        .filter((chat) => chat.conversation_type === 'direct' && chat.peer_id)
        .map((chat) => chat.peer_id!)

    const toggle = (userId: string) =>
        setSelected((ids) =>
            ids.includes(userId) ? ids.filter((id) => id !== userId) : [...ids, userId],
        )

    function submit() {
        create.mutate({
            name: name.trim(),
            description: description.trim() || undefined,
            visibility,
            member_ids: selected,
        })
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
                            {peers.map((userId) => (
                                <PeerCard
                                    key={userId}
                                    userId={userId}
                                    selected={selected.includes(userId)}
                                    onToggle={() => toggle(userId)}
                                />
                            ))}
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
            <div className={styles.photo}>
                <Avatar size={96} />
                <TextLink as="button" className={styles.photoButton} disabled>
                    Выбрать фото
                </TextLink>
            </div>

            <form
                className={styles.fields}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (name.trim()) {
                        setStep('members')
                    }
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

interface PeerCardProps {
    userId: string
    selected: boolean
    onToggle: () => void
}

function PeerCard({ userId, selected, onToggle }: PeerCardProps) {
    const user = useUser(userId)
    const profile = user.data
    if (!profile) {
        return null
    }
    return (
        <div className={styles.card}>
            <UserAvatar src={profile.avatar_url} shape={profile.avatar_shape} size={40} />
            <span className={styles.texts}>
                <span className={styles.name}>{profile.display_name}</span>
                <span className={styles.username}>@{profile.username}</span>
            </span>
            <IconButton
                icon={selected ? 'check' : 'add'}
                size={40}
                aria-label={
                    selected ? `Убрать ${profile.display_name}` : `Добавить ${profile.display_name}`
                }
                aria-pressed={selected}
                className={styles.toggle}
                onClick={onToggle}
            />
        </div>
    )
}
