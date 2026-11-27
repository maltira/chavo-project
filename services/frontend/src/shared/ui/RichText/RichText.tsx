import { Fragment, useEffect, useRef, useState } from 'react'
import type { CSSProperties, ReactNode } from 'react'

import { cn, codeLanguageLabel, normalizeEntities, toSegments, withAutoLinks } from '@/shared/lib'
import type { MessageEntity, Segment } from '@/shared/lib'

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
    const allEntities = withAutoLinks(text, normalizeEntities(entities ?? []))

    if (allEntities.length === 0) {
        return <>{text}</>
    }

    function revealSpoiler(key: number) {
        setRevealed((prev) => {
            if (prev.has(key)) {
                return prev
            }
            const next = new Set(prev)
            next.add(key)
            return next
        })
    }

    function renderLeaf(segment: Segment): ReactNode {
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
        return node
    }

    // группируем смежные сегменты одного спойлера, чтобы вложенный bold/italic не резал спойлер на части
    function renderInlineRun(run: Segment[]): ReactNode[] {
        const out: ReactNode[] = []
        let i = 0
        while (i < run.length) {
            const first = run[i]!
            if (!first.spoiler) {
                out.push(<Fragment key={first.start}>{renderLeaf(first)}</Fragment>)
                i++
                continue
            }
            const spoilerKey = first.spoilerKey ?? first.start
            const group: Segment[] = [first]
            i++
            while (i < run.length && run[i]!.spoiler && (run[i]!.spoilerKey ?? run[i]!.start) === spoilerKey) {
                group.push(run[i]!)
                i++
            }
            const shown = revealed.has(spoilerKey)
            const len = group.reduce((sum, s) => sum + s.text.length, 0)
            const inner = group.map((s) => <Fragment key={s.start}>{renderLeaf(s)}</Fragment>)
            out.push(
                interactive ? (
                    <span
                        key={first.start}
                        role="button"
                        tabIndex={shown ? -1 : 0}
                        aria-label={shown ? undefined : 'Показать скрытый текст'}
                        className={cn(styles.spoiler, shown && styles.spoilerRevealed)}
                        onClick={(event) => {
                            if (!shown) {
                                event.stopPropagation()
                                revealSpoiler(spoilerKey)
                            }
                        }}
                        onKeyDown={(event) => {
                            if (!shown && (event.key === 'Enter' || event.key === ' ')) {
                                event.preventDefault()
                                event.stopPropagation()
                                revealSpoiler(spoilerKey)
                            }
                        }}
                    >
                        <span className={styles.spoilerText}>{inner}</span>
                        <SpoilerMosaic length={len} />
                    </span>
                ) : (
                    <span key={first.start} className={styles.spoiler}>
                        <span className={styles.spoilerText}>{inner}</span>
                        <SpoilerMosaic length={len} />
                    </span>
                ),
            )
        }
        return out
    }

    // группируем смежные сегменты одного блока pre (вложенные стили не разбивают карточку кода)
    function renderPreRuns(run: Segment[]): ReactNode[] {
        const out: ReactNode[] = []
        let i = 0
        while (i < run.length) {
            const first = run[i]!
            if (!first.pre) {
                const inlineGroup: Segment[] = [first]
                i++
                while (i < run.length && !run[i]!.pre) {
                    inlineGroup.push(run[i]!)
                    i++
                }
                out.push(<Fragment key={first.start}>{renderInlineRun(inlineGroup)}</Fragment>)
                continue
            }
            const preKey = first.preKey ?? first.start
            const group: Segment[] = [first]
            i++
            while (i < run.length && run[i]!.pre && (run[i]!.preKey ?? run[i]!.start) === preKey) {
                group.push(run[i]!)
                i++
            }
            const fullCode = group.map((s) => s.text).join('')
            out.push(
                <CodeBlock
                    key={first.start}
                    code={fullCode}
                    language={first.preLanguage}
                    interactive={interactive}
                >
                    {renderInlineRun(group)}
                </CodeBlock>,
            )
        }
        return out
    }

    // верхний уровень: группируем смежные сегменты одной цитаты blockquote
    const segments = toSegments(text, allEntities)
    const nodes: ReactNode[] = []
    let i = 0
    while (i < segments.length) {
        const first = segments[i]!
        if (!first.blockquote) {
            const group: Segment[] = [first]
            i++
            while (i < segments.length && !segments[i]!.blockquote) {
                group.push(segments[i]!)
                i++
            }
            nodes.push(<Fragment key={first.start}>{renderPreRuns(group)}</Fragment>)
            continue
        }
        const quoteKey = first.blockquoteKey ?? first.start
        const group: Segment[] = [first]
        i++
        while (
            i < segments.length &&
            segments[i]!.blockquote &&
            (segments[i]!.blockquoteKey ?? segments[i]!.start) === quoteKey
        ) {
            group.push(segments[i]!)
            i++
        }
        nodes.push(
            <span key={first.start} className={styles.blockquote}>
                {renderPreRuns(group)}
            </span>,
        )
    }

    return <>{nodes}</>
}
