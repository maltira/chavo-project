import styles from './HomePage.module.scss'
import { Button, IconButton, Input, InputCode, Switch, Tag } from '@/shared/ui'

export function HomePage() {
    return (
        <main className={styles.root}>
            <Button variant="secondary" icon="heart">
                Click me
            </Button>
            <Button variant="primary" icon="heart">
                Click me
            </Button>
            <IconButton icon="heart" aria-label="Избранное" />
            <Input placeholder="Enter text" leftIcon="search" rightIcon="eye-open" />
            <div style={{ display: 'flex', gap: '8px' }}>
                <InputCode aria-label="1" />
                <InputCode aria-label="2" />
                <InputCode aria-label="3" />
                <InputCode aria-label="4" />
                <InputCode aria-label="5" />
                <InputCode aria-label="6" />
            </div>

            <Switch aria-label="" />

            <Tag variant="muted">99+</Tag>
        </main>
    )
}
