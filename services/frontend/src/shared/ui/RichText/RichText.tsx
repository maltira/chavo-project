import { Fragment } from 'react'
import type { ReactNode } from 'react'

import { toSegments } from '@/shared/lib'
import type { MessageEntity } from '@/shared/lib'

import styles from './RichText.module.scss'

interface Props {
    text: string
    // форматирование: bold, italic, underline; без него - обычный текст
    entities?: MessageEntity[]
}

// текст сообщения с жирным, курсивом и подчёркиванием; переводы строк сохраняются стилем pre-wrap у родителя
export function RichText({ text, entities }: Props) {
    if (!entities || entities.length === 0) {
        return <>{text}</>
    }
    return (
        <>
            {toSegments(text, entities).map((segment) => {
                let node: ReactNode = segment.text
                if (segment.underline) {
                    node = <u className={styles.underline}>{node}</u>
                }
                if (segment.italic) {
                    node = <em className={styles.italic}>{node}</em>
                }
                if (segment.bold) {
                    node = <strong className={styles.bold}>{node}</strong>
                }
                return <Fragment key={segment.start}>{node}</Fragment>
            })}
        </>
    )
}
