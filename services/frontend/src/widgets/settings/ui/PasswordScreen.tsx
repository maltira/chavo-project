import { ChangePasswordForm } from '@/features/change-password'

// смена пароля; после успеха сеанс завершается и приложение уходит на вход
export function PasswordScreen() {
    return <ChangePasswordForm />
}
