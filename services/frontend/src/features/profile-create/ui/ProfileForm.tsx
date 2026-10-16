import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'

import { profileSchema } from '@/entities/user'
import type { ProfileInput, ProfileValues } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Avatar, Button, Field, Input, Textarea, TextLink } from '@/shared/ui'

import { createProfile } from '../api/createProfile'
import styles from './ProfileForm.module.scss'

const ERRORS: Record<string, string> = {
    USERNAME_TAKEN: 'Это имя пользователя уже занято',
    PROFILE_ALREADY_EXISTS: 'Профиль уже создан',
}

export function ProfileForm() {
    const queryClient = useQueryClient()
    const {
        register,
        handleSubmit,
        control,
        formState: { errors },
    } = useForm<ProfileInput, unknown, ProfileValues>({
        resolver: zodResolver(profileSchema),
        mode: 'onSubmit',
        reValidateMode: 'onChange',
        defaultValues: { display_name: '', username: '', bio: '' },
    })
    const [displayName, username] = useWatch({ control, name: ['display_name', 'username'] })

    const mutation = useMutation({
        mutationFn: createProfile,
        onError: (error) =>
            notify({
                id: 'profile-error',
                icon: 'warn',
                title: 'Не удалось сохранить профиль',
                description: errorMessage(error, ERRORS),
            }),
        // профиль появился: гард сам уведёт на главную, когда ['me'] перестанет быть null
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['me'] }),
    })

    return (
        <form
            className={styles.form}
            noValidate
            onSubmit={handleSubmit((values) =>
                mutation.mutate({
                    display_name: values.display_name,
                    username: values.username,
                    bio: values.bio || undefined,
                }),
            )}
        >
            {/* загрузки фото на сервере пока нет: кнопка неактивна */}
            <div className={styles.photo}>
                <Avatar size={96} />
                <TextLink as="button" className={styles.photoButton} disabled>
                    Выбрать фото
                </TextLink>
            </div>

            <div className={styles.fields}>
                <Field label="Ваше имя" error={errors.display_name?.message}>
                    {(props) => (
                        <Input
                            {...props}
                            autoComplete="name"
                            placeholder="Укажите имя"
                            {...register('display_name')}
                        />
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
                            placeholder="username"
                            {...register('username')}
                        />
                    )}
                </Field>
                <Field label="О себе" error={errors.bio?.message}>
                    {(props) => <Textarea {...props} maxLength={255} {...register('bio')} />}
                </Field>
            </div>

            <Button
                type="submit"
                className={styles.submit}
                disabled={!displayName || !username || mutation.isPending}
            >
                Продолжить
            </Button>
        </form>
    )
}
