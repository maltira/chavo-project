import { IconButton } from '@/shared/ui'

import { useLogout } from '../model/useLogout'

export function LogoutButton() {
    const logout = useLogout()

    return (
        <IconButton
            icon="exit"
            aria-label="Выйти"
            disabled={logout.isPending}
            onClick={() => logout.mutate()}
        />
    )
}
