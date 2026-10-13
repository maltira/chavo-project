import { TELEGRAM_URL } from '@/shared/config'
import { IconButton } from '@/shared/ui'

import styles from './AuthLayout.module.scss'

export function AuthFooter() {
    return (
        <footer className={styles.footer}>
            <a
                href={TELEGRAM_URL}
                target="_blank"
                rel="noreferrer noopener"
                aria-label="Telegram"
                className={styles.telegram}
            >
                <IconButton
                    size={40}
                    icon="shape-telegram"
                    aria-label="Telegram"
                    aria-hidden="true"
                    tabIndex={-1}
                />
            </a>
            <div className={styles.credits}>
                <span className={styles.author}>ENEMYBYE</span>
                <span className={styles.copyright}>@ 2026 enemybye.dev</span>
            </div>
        </footer>
    )
}
