import { chatLinkText } from '@/shared/config'
import { Field, Input } from '@/shared/ui'

import { SLUG_HINT } from '../model/slug'

interface Props {
    value: string
    onChange: (value: string) => void
    // ошибка проверки или ответа сервера (ссылка занята)
    error?: string
}

// код ссылки открытой группы: под полем - готовая ссылка или правило
export function SlugField({ value, onChange, error }: Props) {
    const slug = value.trim()
    return (
        <Field label="Ссылка на чат" error={error} hint={slug ? chatLinkText(slug) : SLUG_HINT}>
            {(props) => (
                <Input
                    {...props}
                    leftIcon="link"
                    placeholder="book_club"
                    maxLength={32}
                    autoComplete="off"
                    spellCheck={false}
                    value={value}
                    // пробелы и кириллицу сервер не примет - не даём их ввести
                    onChange={(event) => onChange(event.target.value.replace(/[^a-zA-Z0-9_]/g, ''))}
                />
            )}
        </Field>
    )
}
