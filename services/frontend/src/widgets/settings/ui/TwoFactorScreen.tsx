import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import {
    confirmTOTP,
    regenerateBackupCodes,
    setupTOTP,
    switchTwoFactorMethod,
    twoFactorKey,
    useTwoFactor,
} from '@/entities/session'
import type { SetupTOTPResponse } from '@/entities/session'
import { errorMessage } from '@/shared/api'
import { cn, notify } from '@/shared/lib'
import { Button, Field, Icon, Input, Loader, PasswordInput, QrCode } from '@/shared/ui'

import common from './SettingsModal.module.scss'
import styles from './TwoFactorScreen.module.scss'

type Step = 'overview' | 'setup-password' | 'setup-confirm' | 'backup-codes' | 'switch-email' | 'regen'

const ERRORS: Record<string, string> = {
    WRONG_PASSWORD: 'Неверный пароль',
    INVALID_CREDENTIALS: 'Неверный пароль',
    INVALID_OTP: 'Неверный код подтверждения',
    TOTP_SETUP_EXPIRED: 'Время настройки истекло, начните заново',
    TOTP_NOT_ENABLED: 'Приложение-аутентификатор не подключено',
    TOTP_ALREADY_ENABLED: 'Приложение-аутентификатор уже подключено',
}

interface Props {
    onCompactChange?: (compact: boolean) => void
}

// управление обязательной 2FA: выбор между кодом на почту и TOTP-приложением + резервные коды
export function TwoFactorScreen({ onCompactChange }: Props) {
    const queryClient = useQueryClient()
    const status = useTwoFactor()

    const [step, setStep] = useState<Step>('overview')
    const [password, setPassword] = useState('')
    const [code, setCode] = useState('')
    const [setup, setSetup] = useState<SetupTOTPResponse | null>(null)
    const [backupCodes, setBackupCodes] = useState<string[]>([])

    useEffect(() => {
        onCompactChange?.(step === 'setup-password' || step === 'switch-email' || step === 'regen')
    }, [step, onCompactChange])

    function resetFields() {
        setPassword('')
        setCode('')
    }

    const startSetup = useMutation({
        mutationFn: () => setupTOTP(password),
        onSuccess: (data) => {
            setSetup(data)
            setPassword('')
            setCode('')
            setStep('setup-confirm')
        },
        onError: (error) =>
            notify({
                id: 'two-factor-error',
                icon: 'warn',
                title: 'Не удалось начать настройку',
                description: errorMessage(error, ERRORS),
            }),
    })

    const finishSetup = useMutation({
        mutationFn: () => confirmTOTP(code.trim()),
        onSuccess: (data) => {
            setBackupCodes(data.backup_codes)
            setSetup(null)
            resetFields()
            void queryClient.invalidateQueries({ queryKey: twoFactorKey })
            notify({
                id: 'two-factor-saved',
                icon: 'check',
                title: 'Приложение-аутентификатор подключено',
            })
            setStep('backup-codes')
        },
        onError: (error) =>
            notify({
                id: 'two-factor-error',
                icon: 'warn',
                title: 'Не удалось подтвердить код',
                description: errorMessage(error, ERRORS),
            }),
    })

    const switchToEmail = useMutation({
        mutationFn: () =>
            switchTwoFactorMethod({
                method: 'email',
                password,
                code: code.trim(),
            }),
        onSuccess: () => {
            resetFields()
            void queryClient.invalidateQueries({ queryKey: twoFactorKey })
            notify({
                id: 'two-factor-saved',
                icon: 'check',
                title: 'Способ входа изменён',
                description: 'Коды подтверждения снова будут приходить на почту',
            })
            setStep('overview')
        },
        onError: (error) =>
            notify({
                id: 'two-factor-error',
                icon: 'warn',
                title: 'Не удалось переключить способ',
                description: errorMessage(error, ERRORS),
            }),
    })

    const regen = useMutation({
        mutationFn: () => regenerateBackupCodes({ password, code: code.trim() }),
        onSuccess: (data) => {
            setBackupCodes(data.backup_codes)
            resetFields()
            void queryClient.invalidateQueries({ queryKey: twoFactorKey })
            notify({
                id: 'two-factor-saved',
                icon: 'check',
                title: 'Резервные коды обновлены',
            })
            setStep('backup-codes')
        },
        onError: (error) =>
            notify({
                id: 'two-factor-error',
                icon: 'warn',
                title: 'Не удалось обновить коды',
                description: errorMessage(error, ERRORS),
            }),
    })

    function copyText(text: string, title: string) {
        if (!navigator.clipboard) {
            return
        }
        void navigator.clipboard.writeText(text).then(() => {
            notify({ id: 'copied', icon: 'check', title })
        })
    }

    if (status.isPending) {
        return (
            <div className={common.empty}>
                <Loader />
            </div>
        )
    }

    if (status.isError || !status.data) {
        return (
            <div className={common.body}>
                <p className={common.empty}>Не удалось загрузить настройки защиты</p>
                <Button
                    variant="secondary"
                    className={common.footer}
                    onClick={() => void status.refetch()}
                >
                    Повторить
                </Button>
            </div>
        )
    }

    if (step === 'setup-password') {
        return (
            <form
                className={common.body}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (password) {
                        startSetup.mutate()
                    }
                }}
            >
                <Field
                    label="Текущий пароль"
                    hint="Для подключения приложения-аутентификатора подтвердите текущий пароль"
                >
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoFocus
                            autoComplete="current-password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                        />
                    )}
                </Field>
                <div className={styles.footerActions}>
                    <Button
                        type="submit"
                        disabled={!password || startSetup.isPending}
                    >
                        Продолжить
                    </Button>
                    <Button
                        type="button"
                        variant="secondary"
                        disabled={startSetup.isPending}
                        onClick={() => {
                            resetFields()
                            setStep('overview')
                        }}
                    >
                        Отмена
                    </Button>
                </div>
            </form>
        )
    }

    if (step === 'setup-confirm' && setup) {
        const cleanCode = code.replace(/\D/g, '')
        return (
            <form
                className={common.body}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (cleanCode.length === 6) {
                        finishSetup.mutate()
                    }
                }}
            >
                <p className={common.hint}>
                    Отсканируйте QR-код в Google Authenticator, 1Password или другом
                    TOTP-приложении, либо введите ключ вручную.
                </p>

                <div className={styles.qrBox}>
                    <QrCode
                        value={setup.otpauth_url}
                        size={176}
                        label="QR-код для приложения-аутентификатора"
                    />
                </div>

                <div className={styles.secretRow}>
                    <code className={styles.secret}>{setup.secret}</code>
                    <Button
                        type="button"
                        variant="secondary"
                        className={styles.copyBtn}
                        aria-label="Скопировать секретный ключ"
                        onClick={() => copyText(setup.secret, 'Ключ скопирован')}
                    >
                        <Icon name="copy" size={18} color="inherit" />
                    </Button>
                </div>

                <Field label="Код из приложения">
                    {(props) => (
                        <Input
                            {...props}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            placeholder="000000"
                            maxLength={6}
                            value={code}
                            onChange={(event) =>
                                setCode(event.target.value.replace(/\D/g, '').slice(0, 6))
                            }
                        />
                    )}
                </Field>

                <div className={styles.footerActions}>
                    <Button
                        type="submit"
                        disabled={cleanCode.length !== 6 || finishSetup.isPending}
                    >
                        Подтвердить
                    </Button>
                    <Button
                        type="button"
                        variant="secondary"
                        disabled={finishSetup.isPending}
                        onClick={() => {
                            setSetup(null)
                            resetFields()
                            setStep('overview')
                        }}
                    >
                        Отмена
                    </Button>
                </div>
            </form>
        )
    }

    if (step === 'backup-codes') {
        return (
            <div className={common.body}>
                <div className={styles.warning}>
                    <Icon name="warn" size={20} color="inherit" />
                    <span>
                        Сохраните эти резервные коды в надёжном месте. Они показываются только один
                        раз и позволяют войти, если потеряете доступ к приложению.
                    </span>
                </div>

                <div className={styles.codesGrid}>
                    {backupCodes.map((item) => (
                        <code key={item} className={styles.codeItem}>
                            {item}
                        </code>
                    ))}
                </div>

                <div className={styles.footerActions}>
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={() =>
                            copyText(backupCodes.join('\n'), 'Резервные коды скопированы')
                        }
                    >
                        <Icon name="copy" size={18} color="inherit" />
                        Скопировать все
                    </Button>
                    <Button
                        type="button"
                        onClick={() => {
                            setBackupCodes([])
                            setStep('overview')
                        }}
                    >
                        Готово
                    </Button>
                </div>
            </div>
        )
    }

    if (step === 'switch-email') {
        return (
            <form
                className={common.body}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (password && code.trim().length >= 6) {
                        switchToEmail.mutate()
                    }
                }}
            >
                <Field label="Текущий пароль">
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoFocus
                            autoComplete="current-password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                        />
                    )}
                </Field>
                <Field
                    label="Код из приложения или резервный код"
                    hint="Коды входа снова будут приходить на почту, а ключ приложения и резервные коды удалятся"
                >
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
                <div className={styles.footerActions}>
                    <Button
                        type="submit"
                        disabled={!password || code.trim().length < 6 || switchToEmail.isPending}
                    >
                        Переключить на почту
                    </Button>
                    <Button
                        type="button"
                        variant="secondary"
                        disabled={switchToEmail.isPending}
                        onClick={() => {
                            resetFields()
                            setStep('overview')
                        }}
                    >
                        Отмена
                    </Button>
                </div>
            </form>
        )
    }

    if (step === 'regen') {
        const cleanCode = code.replace(/\D/g, '')
        return (
            <form
                className={common.body}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (password && cleanCode.length === 6) {
                        regen.mutate()
                    }
                }}
            >
                <Field label="Текущий пароль">
                    {(props) => (
                        <PasswordInput
                            {...props}
                            autoFocus
                            autoComplete="current-password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                        />
                    )}
                </Field>
                <Field
                    label="Код из приложения"
                    hint="Будет выпущено 8 новых резервных кодов — прежние сразу перестанут действовать"
                >
                    {(props) => (
                        <Input
                            {...props}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            placeholder="000000"
                            maxLength={6}
                            value={code}
                            onChange={(event) =>
                                setCode(event.target.value.replace(/\D/g, '').slice(0, 6))
                            }
                        />
                    )}
                </Field>
                <div className={styles.footerActions}>
                    <Button
                        type="submit"
                        disabled={!password || cleanCode.length !== 6 || regen.isPending}
                    >
                        Выпустить новые коды
                    </Button>
                    <Button
                        type="button"
                        variant="secondary"
                        disabled={regen.isPending}
                        onClick={() => {
                            resetFields()
                            setStep('overview')
                        }}
                    >
                        Отмена
                    </Button>
                </div>
            </form>
        )
    }

    const isTotp = status.data.method === 'totp'

    return (
        <div className={common.body}>
            <div
                role="radiogroup"
                aria-label="Способ подтверждения входа"
                className={common.card}
            >
                <button
                    type="button"
                    role="radio"
                    aria-checked={!isTotp}
                    className={cn(styles.methodOption, !isTotp && styles.chosen)}
                    onClick={() => {
                        if (isTotp) {
                            resetFields()
                            setStep('switch-email')
                        }
                    }}
                >
                    <span className={styles.radio} aria-hidden="true" />
                    <span className={styles.statusInfo}>
                        <span className={styles.statusTitle}>Электронная почта</span>
                        <span className={styles.statusSub}>
                            Одноразовые коды отправляются на привязанную почту
                        </span>
                    </span>
                </button>

                <button
                    type="button"
                    role="radio"
                    aria-checked={isTotp}
                    className={cn(styles.methodOption, isTotp && styles.chosen)}
                    onClick={() => {
                        if (!isTotp) {
                            resetFields()
                            setStep('setup-password')
                        }
                    }}
                >
                    <span className={styles.radio} aria-hidden="true" />
                    <span className={styles.statusInfo}>
                        <span className={styles.statusTitle}>Приложение-аутентификатор</span>
                        <span className={styles.statusSub}>
                            6-значные коды в Google Authenticator, 1Password и других TOTP-приложениях
                        </span>
                    </span>
                </button>
            </div>

            <p className={common.hint}>
                Двухфакторная защита обязательна для всех аккаунтов и не отключается. Выберите
                удобный способ получения одноразового кода.
            </p>

            {isTotp && (
                <>
                    <div className={styles.footerActions}>
                        <Button
                            type="button"
                            variant="secondary"
                            onClick={() => {
                                resetFields()
                                setStep('regen')
                            }}
                        >
                            Обновить резервные коды
                        </Button>
                    </div>
                </>
            )}
        </div>
    )
}
