import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, useWatch } from 'react-hook-form'

import { profileSchema, uploadMyAvatar } from '@/entities/user'
import type { ProfileInput, ProfileValues } from '@/entities/user'
import { errorMessage } from '@/shared/api'
import { notify, usePhotoDraft } from '@/shared/lib'
import { Avatar, Button, Field, Input, PhotoPicker, Textarea, TextLink } from '@/shared/ui'

import { createProfile } from '../api/createProfile'
import type { CreateProfileBody } from '../api/createProfile'
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
    // фото можно загрузить только когда профиль уже есть: до этого оно ждёт здесь
    const draft = usePhotoDraft()

    const mutation = useMutation({
        mutationFn: async (body: CreateProfileBody) => {
            await createProfile(body)
            if (draft.photo) {
                try {
                    await uploadMyAvatar(draft.photo)
                } catch (error) {
                    // профиль уже создан - не блокируем вход из-за фото
                    notify({
                        icon: 'warn',
                        title: 'Фото не загрузилось',
                        description: `${errorMessage(error)}. Добавьте его позже в профиле`,
                    })
                }
            }
        },
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
            {/* фото уйдёт на сервер сразу после создания профиля */}
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
