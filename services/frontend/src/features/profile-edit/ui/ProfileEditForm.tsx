import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'

import { profileSchema, updateMe } from '@/entities/user'
import type { Profile, ProfileInput, ProfileUpdate, ProfileValues } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Avatar, Button, Field, Input, Textarea, TextLink } from '@/shared/ui'

import styles from './ProfileEditForm.module.scss'

const ERRORS: Record<string, string> = {
    USERNAME_TAKEN: 'Это имя пользователя уже занято',
}

interface Props {
    profile: Profile
    // сохранено или передумали - вернуться к просмотру профиля
    onDone: () => void
}

export function ProfileEditForm({ profile: current, onDone }: Props) {
    // сравниваем с профилем на момент открытия формы: проп может обновиться, пока форма открыта
    const [profile] = useState(current)
    const queryClient = useQueryClient()
    const {
        register,
        handleSubmit,
        setError,
        formState: { errors, isDirty },
    } = useForm<ProfileInput, unknown, ProfileValues>({
        resolver: zodResolver(profileSchema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
        defaultValues: {
            display_name: profile.display_name,
            username: profile.username,
            bio: profile.bio ?? '',
        },
    })

    const save = useMutation({
        mutationFn: (update: ProfileUpdate) => updateMe(update),
        onSuccess: async () => {
            await queryClient.invalidateQueries({ queryKey: ['me'] })
            notify({ icon: 'check', title: 'Профиль сохранён' })
            onDone()
        },
        onError: (error) => {
            const message = errorMessage(error, ERRORS)
            if (message === ERRORS.USERNAME_TAKEN) {
                setError('username', { message })
                return
            }
            notify({ icon: 'warn', title: 'Не удалось сохранить профиль', description: message })
        },
    })

    function submit(values: ProfileValues) {
        const update: ProfileUpdate = {}
        if (values.display_name !== profile.display_name) update.display_name = values.display_name
        if (values.username !== profile.username) update.username = values.username
        if (values.bio !== (profile.bio ?? '')) update.bio = values.bio
        if (Object.keys(update).length === 0) {
            onDone()
            return
        }
        save.mutate(update)
    }

    return (
        <form className={styles.form} noValidate onSubmit={handleSubmit(submit)}>
            {/* загрузки фото на сервере пока нет: кнопка неактивна */}
            <div className={styles.photo}>
                <Avatar src={profile.avatar_url} size={96} />
                <TextLink as="button" className={styles.photoButton} disabled>
                    Выбрать фото
                </TextLink>
            </div>

            <div className={styles.fields}>
                <Field label="Ваше имя" error={errors.display_name?.message}>
                    {(props) => (
                        <Input {...props} autoComplete="name" {...register('display_name')} />
                    )}
                </Field>
                <Field
                    label="Имя пользователя"
                    hint="По имени пользователя другие люди смогут находить вас и связываться с вами."
                    error={errors.username?.message}
                >
                    {(props) => (
                        <Input
                            {...props}
                            leftIcon="at"
                            autoComplete="username"
                            {...register('username')}
                        />
                    )}
                </Field>
                <Field label="О себе" error={errors.bio?.message}>
                    {(props) => <Textarea {...props} maxLength={255} {...register('bio')} />}
                </Field>
            </div>

            <Button type="submit" variant="secondary" disabled={!isDirty || save.isPending}>
                Сохранить изменения
            </Button>
        </form>
    )
}
