import { Fragment, useEffect, useRef, useState } from 'react'
import type { CSSProperties, ReactNode } from 'react'

import { cn, codeLanguageLabel, toSegments, withAutoLinks } from '@/shared/lib'
import type { MessageEntity } from '@/shared/lib'

import { Icon } from '../Icon'
import styles from './RichText.module.scss'

function CodeBlock({
    code,
    language,
    interactive,
    children,
}: {
    code: string
    language?: string
    interactive: boolean
    children: ReactNode
}) {
    const [copied, setCopied] = useState(false)
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    useEffect(
        () => () => {
            if (timerRef.current !== null) {
                clearTimeout(timerRef.current)
            }
        },
        [],
    )

    function handleCopy() {
        if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
            void navigator.clipboard.writeText(code)
        }
        setCopied(true)
        if (timerRef.current !== null) {
            clearTimeout(timerRef.current)
        }
        timerRef.current = setTimeout(() => {
            setCopied(false)
            timerRef.current = null
        }, 1500)
    }

    if (!interactive) {
        return (
            <code className={styles.pre} data-lang={language}>
                {children}
            </code>
        )
    }

    return (
        <span className={styles.preBlock} data-lang={language}>
            <span className={styles.preHeader}>
                <span className={styles.preLang}>{codeLanguageLabel(language)}</span>
                <button
                    type="button"
                    className={styles.preCopy}
                    aria-label="Копировать код"
                    onClick={(event) => {
                        event.stopPropagation()
                        handleCopy()
                    }}
                >
                    <Icon name={copied ? 'check' : 'copy'} size={14} color="currentColor" />
                    <span>{copied ? 'Скопировано' : 'Копировать'}</span>
                </button>
            </span>
            <code className={styles.preCode}>{children}</code>
        </span>
    )
}

function SpoilerMosaic({ length }: { length: number }) {
    const cols = Math.max(4, Math.min(20, Math.round(length * 0.7)))
    const total = cols * 2
    return (
        <span
            aria-hidden="true"
            className={styles.spoilerCover}
            style={{ '--spoiler-cols': cols } as CSSProperties}
        >
            {Array.from({ length: total }, (_, i) => {
                const col = i % cols
                const strength = `${38 + ((i * 37) % 55)}%`
                const idleDelay = `${(i * 190) % 2400}ms`
                const outDelay = `${Math.round((col / cols) * 180 + (i >= cols ? 35 : 0))}ms`
                return (
                    <span
                        key={i}
                        className={styles.spoilerCell}
                        style={
                            {
                                '--cell-strength': strength,
                                '--idle-delay': idleDelay,
                                '--out-delay': outDelay,
                            } as CSSProperties
                        }
                    />
                )
            })}
        </span>
    )
}

interface Props {
    text: string
    // форматирование текста; без него - обычный текст (с автолинковкой URL)
    entities?: MessageEntity[]
    // false внутри кнопок-цитат и плашек превью, чтобы не вкладывать интерактивные элементы
    interactive?: boolean
    // клик по упоминанию @username открывает профиль пользователя
    onMentionClick?: (userId: string) => void
}

// текст сообщения с расширенным форматированием, ссылками, спойлерами и упоминаниями;
// переводы строк сохраняются стилем pre-wrap у родителя
export function RichText({ text, entities, interactive = true, onMentionClick }: Props) {
    const [revealed, setRevealed] = useState<Set<number>>(() => new Set())
    const allEntities = withAutoLinks(text, entities ?? [])

    if (allEntities.length === 0) {
        return <>{text}</>
    }

    function revealSpoiler(start: number) {
        setRevealed((prev) => {
            if (prev.has(start)) {
                return prev
            }
            const next = new Set(prev)
            next.add(start)
            return next
        })
    }

    return (
        <>
            {toSegments(text, allEntities).map((segment) => {
                let node: ReactNode = segment.text
                if (segment.code) {
                    node = <code className={styles.code}>{node}</code>
                }
                if (segment.strikethrough) {
                    node = <s className={styles.strikethrough}>{node}</s>
                }
                if (segment.underline) {
                    node = <u className={styles.underline}>{node}</u>
                }
                if (segment.italic) {
                    node = <em className={styles.italic}>{node}</em>
                }
                if (segment.bold) {
                    node = <strong className={styles.bold}>{node}</strong>
                }
                if (segment.linkUrl) {
                    node = interactive ? (
                        <a
                            href={segment.linkUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={styles.link}
                            onClick={(event) => event.stopPropagation()}
                        >
                            {node}
                        </a>
                    ) : (
                        <span className={styles.link}>{node}</span>
                    )
                }
                if (segment.mentionUserId) {
                    const userId = segment.mentionUserId
                    node =
                        interactive && onMentionClick ? (
                            <button
                                type="button"
                                className={styles.mention}
                                onClick={(event) => {
                                    event.stopPropagation()
                                    onMentionClick(userId)
                                }}
                            >
                                {node}
                            </button>
                        ) : (
                            <span className={styles.mention}>{node}</span>
                        )
                }
                if (segment.spoiler) {
                    const shown = revealed.has(segment.start)
                    const len = segment.text.length
                    node = interactive ? (
                        <span
                            role="button"
                            tabIndex={shown ? -1 : 0}
                            aria-label={shown ? undefined : 'Показать скрытый текст'}
                            className={cn(styles.spoiler, shown && styles.spoilerRevealed)}
                            onClick={(event) => {
                                if (!shown) {
                                    event.stopPropagation()
                                    revealSpoiler(segment.start)
                                }
                            }}
                            onKeyDown={(event) => {
                                if (!shown && (event.key === 'Enter' || event.key === ' ')) {
                                    event.preventDefault()
                                    event.stopPropagation()
                                    revealSpoiler(segment.start)
                                }
                            }}
                        >
                            <span className={styles.spoilerText}>{node}</span>
                            <SpoilerMosaic length={len} />
                        </span>
                    ) : (
                        <span className={styles.spoiler}>
                            <span className={styles.spoilerText}>{node}</span>
                            <SpoilerMosaic length={len} />
                        </span>
                    )
                }
                if (segment.pre) {
                    node = (
                        <CodeBlock
                            code={segment.text}
                            language={segment.preLanguage}
                            interactive={interactive}
                        >
                            {node}
                        </CodeBlock>
                    )
                }
                if (segment.blockquote) {
                    node = <span className={styles.blockquote}>{node}</span>
                }
                return <Fragment key={segment.start}>{node}</Fragment>
            })}
        </>
    )
}
