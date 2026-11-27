import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { useDrafts } from '@/entities/message'
import { deleteAccount, useSession, useTwoFactor } from '@/entities/session'
import { errorMessage } from '@/shared/api'
import { notify } from '@/shared/lib'
import { Button, ConfirmModal, Field, Icon, Input, Modal, PasswordInput } from '@/shared/ui'

import styles from './DeleteAccountScreen.module.scss'
import common from './SettingsModal.module.scss'

const ERRORS: Record<string, string> = {
    WRONG_PASSWORD: 'Неверный пароль',
    INVALID_CREDENTIALS: 'Неверный пароль',
    INVALID_OTP: 'Неверный код подтверждения',
}

interface Props {
    open: boolean
    onClose: () => void
    onBack: () => void
}

// необратимое удаление аккаунта с подтверждением пароля (и TOTP/резервного кода при активном TOTP)
export function DeleteAccountScreen({ open, onClose, onBack }: Props) {
    const queryClient = useQueryClient()
    const signOut = useSession((state) => state.signOut)
    const twoFactor = useTwoFactor()

    const [password, setPassword] = useState('')
    const [code, setCode] = useState('')
    const [confirming, setConfirming] = useState(false)

    const needsCode = twoFactor.data?.method === 'totp'
    const canSubmit = !!password && (!needsCode || code.trim().length >= 6)

    const remove = useMutation({
        mutationFn: () =>
            deleteAccount({
                password,
                code: needsCode ? code.trim() : undefined,
            }),
        onSuccess: () => {
            setConfirming(false)
            useDrafts.getState().clearAll()
            notify({
                id: 'account-deleted',
                icon: 'trash',
                title: 'Аккаунт удалён',
                description: 'Ваш аккаунт был удалён навсегда',
            })
            queryClient.clear()
            signOut()
        },
        onError: (error) => {
            setConfirming(false)
            notify({
                id: 'delete-account-error',
                icon: 'warn',
                title: 'Не удалось удалить аккаунт',
                description: errorMessage(error, ERRORS),
            })
        },
    })

    if (confirming) {
        return (
            <ConfirmModal
                key="delete-account-confirm"
                open={open}
                onClose={() => setConfirming(false)}
                onConfirm={() => remove.mutate()}
                title="Удалить аккаунт навсегда?"
                confirmLabel="Удалить"
                danger
                pending={remove.isPending}
            >
                Восстановить аккаунт после удаления будет невозможно.
            </ConfirmModal>
        )
    }

    return (
        <Modal
            key="delete-account"
            open={open}
            onClose={onClose}
            title="Удаление аккаунта"
            headerStart={
                <button
                    type="button"
                    className={common.back}
                    aria-label="Назад"
                    onClick={onBack}
                >
                    <Icon name="arrow-left" size={24} color="inherit" />
                </button>
            }
        >
            <form
                className={common.body}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (canSubmit) {
                        setConfirming(true)
                    }
                }}
            >
                <Field label="Текущий пароль">
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoComplete="current-password"
                            placeholder="Введите пароль"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                        />
                    )}
                </Field>

                {needsCode && (
                    <Field label="Код из приложения или резервный код">
                        {(props) => (
                            <Input
                                {...props}
                                autoComplete="off"
                                placeholder="000000 или XXXX-XXXX"
                                value={code}
                                onChange={(event) => setCode(event.target.value.toUpperCase())}
                            />
                        )}
                    </Field>
                )}

                <p className={styles.warning}>
                    Аккаунт будет удалён безвозвратно — восстановить его будет невозможно.
                </p>

                <Button
                    type="submit"
                    variant="danger"
                    className={common.footer}
                    disabled={!canSubmit || remove.isPending}
                >
                    <Icon name="trash" size={20} color="inherit" />
                    Удалить аккаунт навсегда
                </Button>
            </form>
        </Modal>
    )
}
