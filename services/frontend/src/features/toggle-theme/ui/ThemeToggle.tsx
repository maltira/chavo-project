import { IconButton } from '@/shared/ui'

import { useTheme } from '../model/theme'

export function ThemeToggle() {
    const { theme, toggle } = useTheme()
    const isDark = theme === 'dark'

    return (
        <IconButton
            size={40}
            icon={isDark ? 'theme-dark' : 'theme-light'}
            aria-label={isDark ? 'Включить светлую тему' : 'Включить тёмную тему'}
            onClick={toggle}
        />
    )
}
