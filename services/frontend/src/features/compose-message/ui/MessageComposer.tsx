import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'

import { searchMembers, sendTyping, TYPING_SEND_INTERVAL_MS } from '@/entities/conversation'
import { getDraft, getLinkPreview, MAX_MESSAGE_LENGTH, useDrafts } from '@/entities/message'
import type { Message } from '@/entities/message'
import { useFavoriteEmojis, UserAvatar } from '@/entities/user'
import type { Profile } from '@/entities/user'
import {
    cn,
    extractFirstUrl,
    sameValue,
    trimValue,
    useDebouncedValue,
    useDismiss,
} from '@/shared/lib'
import type { RichValue } from '@/shared/lib'
import { EmojiPicker, Icon, IconButton, RichEditor } from '@/shared/ui'
import type { RichEditorHandle } from '@/shared/ui'

import { useEditMessage } from '../model/useEditMessage'
import { useSendMessage } from '../model/useSendMessage'
import styles from './MessageComposer.module.scss'

export interface ReplyTarget {
    id: string
    author: string
    text: string
}

interface Props {
    // куда писать: существующий чат или человек, с которым чата ещё нет
    conversationId?: string
    recipientId?: string
    myId: string | undefined
    // в группах работает автодополнение упоминаний по @
    isGroup?: boolean
    // сервер принял сообщение (для нового чата - в нём уже есть conversation_id)
    onSent?: (message: Message) => void
    replyTo?: ReplyTarget | null
    // редактируемое сообщение; при смене родитель пересоздаёт компонент через key
    editing?: Message | null
    onCancel: () => void
}

const EMPTY: RichValue = { text: '', entities: [] }
const SEND_LONG_PRESS_MS = 450
const MENTION_TRIGGER_RE = /(?:^|[\s([{"'«])@([a-zA-Z0-9_а-яА-ЯёЁ.-]{0,32})$/
const MAX_MENTION_SUGGESTIONS = 6

interface MentionQuery {
    start: number
    end: number
    query: string
}

function detectMention(text: string, caret: number | null): MentionQuery | null {
    if (caret === null || caret <= 0) {
        return null
    }
    const before = text.slice(0, caret)
    const match = MENTION_TRIGGER_RE.exec(before)
    if (!match) {
        return null
    }
    const query = match[1] ?? ''
    const start = caret - query.length - 1
    return { start, end: caret, query }
}

// текст и форматирование сообщения, которое правят
function valueOf(message: Message | null | undefined): RichValue {
    return { text: message?.content ?? '', entities: message?.entities ?? [] }
}

// сохранённый черновик чата
function draftValue(chatKey: string | undefined): RichValue {
    const draft = getDraft(chatKey)
    return draft ? { text: draft.text, entities: draft.entities } : EMPTY
}

// поле ввода сообщения: отправка, ответ, редактирование, расширенное форматирование, упоминания @ в группах
// и превью ссылок перед отправкой
export function MessageComposer({
    conversationId,
    recipientId,
    myId,
    isGroup = false,
    onSent,
    replyTo,
    editing,
    onCancel,
}: Props) {
    const draftKey = conversationId ?? (recipientId ? `new:${recipientId}` : undefined)
    const setDraft = useDrafts((state) => state.setDraft)
    const clearDraft = useDrafts((state) => state.clearDraft)
    const [initial] = useState<RichValue>(() =>
        editing ? valueOf(editing) : draftValue(draftKey),
    )
    const [value, setValue] = useState<RichValue>(initial)
    const valueRef = useRef(value)
    const editorRef = useRef<RichEditorHandle>(null)
    const send = useSendMessage(myId, onSent)

    // панель эмодзи: кнопка и панель - одна область, клик вне неё и Esc закрывают панель
    const [emojiOpen, setEmojiOpen] = useState(false)
    const emojiRef = useRef<HTMLDivElement>(null)
    const closeEmoji = useCallback(() => setEmojiOpen(false), [setEmojiOpen])
    useDismiss(emojiOpen, emojiRef, closeEmoji)
    const favorites = useFavoriteEmojis(emojiOpen)

    // меню «Отправить без звука» по правому клику или долгому нажатию на кнопку отправки
    const [silentMenuOpen, setSilentMenuOpen] = useState(false)
    const sendWrapRef = useRef<HTMLDivElement>(null)
    const closeSilentMenu = useCallback(() => setSilentMenuOpen(false), [setSilentMenuOpen])
    useDismiss(silentMenuOpen, sendWrapRef, closeSilentMenu)
    const sendPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const suppressClickRef = useRef(false)
    const lastTypingRef = useRef(0)

    // автодополнение упоминаний по @ в группах: серверный поиск по всему составу (до 5000 участников)
    const [mention, setMention] = useState<(MentionQuery & { index: number }) | null>(null)
    const debouncedMentionQuery = useDebouncedValue(mention ? mention.query.trim() : null, 120)
    const mentionQuery = useQuery({
        queryKey: ['member-search', conversationId, debouncedMentionQuery],
        queryFn: () =>
            searchMembers(conversationId!, debouncedMentionQuery!, MAX_MENTION_SUGGESTIONS, true),
        enabled: isGroup && !!conversationId && !!mention && debouncedMentionQuery !== null,
        staleTime: 30_000,
        placeholderData: keepPreviousData,
    })
    const mentionSuggestions = useMemo<Profile[]>(
        () =>
            mention
                ? (mentionQuery.data ?? []).filter((p) => !p.is_deleted && p.user_id !== myId)
                : [],
        [mention, mentionQuery.data, myId],
    )

    // живое превью первой ссылки в сообщении с возможностью скрыть его перед отправкой
    const activeUrl = useMemo(() => extractFirstUrl(value), [value])
    const debouncedUrl = useDebouncedValue(activeUrl, 350)
    const [dismissedUrl, setDismissedUrl] = useState<string | null>(() =>
        editing && !editing.link_preview ? extractFirstUrl(valueOf(editing)) : null,
    )
    const previewDisabled = !!activeUrl && dismissedUrl === activeUrl
    const previewQuery = useQuery({
        queryKey: ['link-preview', debouncedUrl],
        queryFn: () => getLinkPreview(debouncedUrl!),
        enabled: !!debouncedUrl && dismissedUrl !== debouncedUrl,
        staleTime: 10 * 60_000,
        retry: false,
    })
    const existingPreview =
        editing?.link_preview && editing.link_preview.url === activeUrl
            ? editing.link_preview
            : null
    const preview =
        !previewDisabled && debouncedUrl === activeUrl
            ? (previewQuery.data ?? existingPreview)
            : null

    function clearSendLongPress() {
        if (sendPressTimerRef.current !== null) {
            clearTimeout(sendPressTimerRef.current)
            sendPressTimerRef.current = null
        }
    }

    useEffect(() => clearSendLongPress, [])

    // при изменении ответа синхронизируем черновик (после первого рендера, чтобы не писать пустой)
    const replyMountedRef = useRef(false)
    useEffect(() => {
        if (!replyMountedRef.current) {
            replyMountedRef.current = true
            return
        }
        if (!editing && draftKey) {
            setDraft(draftKey, {
                text: valueRef.current.text,
                entities: valueRef.current.entities,
                replyTo: replyTo ?? null,
            })
        }
    }, [replyTo, editing, draftKey, setDraft])

    function handleChange(next: RichValue) {
        valueRef.current = next
        setValue(next)
        if (!editing && draftKey) {
            setDraft(draftKey, {
                text: next.text,
                entities: next.entities,
                replyTo: replyTo ?? null,
            })
        }
        if (!editing && conversationId && next.text.trim().length > 0) {
            const now = Date.now()
            if (now - lastTypingRef.current >= TYPING_SEND_INTERVAL_MS) {
                lastTypingRef.current = now
                sendTyping(conversationId)
            }
        }
    }

    const handleCaretChange = useCallback(
        (val: RichValue, caret: number | null) => {
            if (!isGroup) {
                return
            }
            const next = detectMention(val.text, caret)
            setMention((prev) => {
                if (!next) {
                    return prev ? null : prev
                }
                if (
                    prev &&
                    prev.start === next.start &&
                    prev.end === next.end &&
                    prev.query === next.query
                ) {
                    return prev
                }
                return { ...next, index: 0 }
            })
        },
        [isGroup, setMention],
    )

    function pickMention(profile: Profile) {
        if (!mention) {
            return
        }
        const label = `@${profile.username || profile.display_name}`
        editorRef.current?.insertMention(mention.start, mention.end, label, profile.user_id)
        setMention(null)
    }

    function handleSpecialKeyDown(event: KeyboardEvent<HTMLDivElement>): boolean {
        if (!mention || mentionSuggestions.length === 0) {
            return false
        }
        if (event.key === 'ArrowDown') {
            event.preventDefault()
            setMention((prev) =>
                prev ? { ...prev, index: (prev.index + 1) % mentionSuggestions.length } : prev,
            )
            return true
        }
        if (event.key === 'ArrowUp') {
            event.preventDefault()
            setMention((prev) =>
                prev
                    ? {
                          ...prev,
                          index:
                              (prev.index - 1 + mentionSuggestions.length) %
                              mentionSuggestions.length,
                      }
                    : prev,
            )
            return true
        }
        if (event.key === 'Enter' || event.key === 'Tab') {
            event.preventDefault()
            const chosen = mentionSuggestions[mention.index % mentionSuggestions.length]
            if (chosen) {
                pickMention(chosen)
            }
            return true
        }
        if (event.key === 'Escape') {
            event.preventDefault()
            setMention(null)
            return true
        }
        return false
    }

    // при открытии чата фокуса нет - пользователь сам нажмёт на поле;
    // при редактировании - фокус и курсор в конце текста
    const editingId = editing?.id
    useEffect(() => {
        if (editingId) {
            editorRef.current?.focus()
            const edited = valueOf(editing)
            editorRef.current?.setValue(edited)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps -- содержимое берётся при смене сообщения
    }, [editingId])

    // выбрали сообщение для ответа - курсор сразу в поле, можно печатать
    const replyId = replyTo?.id
    useEffect(() => {
        if (replyId) {
            editorRef.current?.focus()
        }
    }, [replyId])
    const edit = useEditMessage()

    const trimmed = trimValue(value)
    const previewChanged =
        !!editing && Boolean(editing.link_preview) === previewDisabled && !!activeUrl
    const canSubmit =
        trimmed.text.length > 0 &&
        (!editing || !sameValue(trimmed, trimValue(valueOf(editing))) || previewChanged)

    function submit(silent = false) {
        if (!canSubmit) {
            return
        }
        setSilentMenuOpen(false)
        setMention(null)
        if (editing) {
            edit.mutate({
                id: editing.id,
                content: trimmed.text,
                entities: trimmed.entities,
                ...(previewDisabled ? { disable_link_preview: true } : {}),
            })
            onCancel()
            return
        }
        lastTypingRef.current = 0
        send.mutate({
            ...(conversationId
                ? { conversation_id: conversationId }
                : { recipient_id: recipientId }),
            content: trimmed.text,
            ...(trimmed.entities.length > 0 ? { entities: trimmed.entities } : {}),
            ...(silent ? { is_silent: true } : {}),
            ...(previewDisabled ? { disable_link_preview: true } : {}),
            reply_to_message_id: replyTo?.id ?? null,
        })
        // поле и черновик очищаются, форматирование сбрасывается
        if (draftKey) {
            clearDraft(draftKey)
        }
        setDismissedUrl(null)
        valueRef.current = EMPTY
        editorRef.current?.setValue(EMPTY)
        onCancel()
    }

    // эмодзи встаёт в позицию курсора (или заменяет выделенное). После выбора с клавиатуры фокус остаётся
    // в панели, чтобы можно было выбрать следующий; после выбора мышью - в поле, чтобы печатать дальше
    function insertEmoji(emoji: string, options?: { keyboard: boolean }) {
        const active = document.activeElement as HTMLElement | null
        editorRef.current?.insertText(emoji)
        if (options?.keyboard) {
            active?.focus()
        }
    }

    const context = editing
        ? { title: 'Редактирование', text: editing.content ?? '' }
        : replyTo
          ? { title: `${replyTo.author}:`, text: replyTo.text }
          : null

    return (
        <div className={styles.root}>
            {mention && mentionSuggestions.length > 0 && (
                <div
                    role="listbox"
                    aria-label="Упомянуть участника"
                    className={styles.mentions}
                    onMouseDown={(event) => event.preventDefault()}
                >
                    {mentionSuggestions.map((profile, idx) => {
                        const selected = idx === mention.index % mentionSuggestions.length
                        return (
                            <button
                                key={profile.user_id}
                                type="button"
                                role="option"
                                aria-selected={selected}
                                className={cn(
                                    styles.mentionItem,
                                    selected && styles.mentionItemActive,
                                )}
                                onClick={() => pickMention(profile)}
                            >
                                <UserAvatar
                                    src={profile.avatar_url}
                                    shape={profile.avatar_shape}
                                    size={28}
                                />
                                <span className={styles.mentionName}>{profile.display_name}</span>
                                {profile.username && (
                                    <span className={styles.mentionUsername}>
                                        @{profile.username}
                                    </span>
                                )}
                            </button>
                        )
                    })}
                </div>
            )}

            {preview && (
                <div className={styles.linkPreview} aria-label="Превью ссылки">
                    <span className={styles.linkPreviewBar} aria-hidden="true" />
                    <div className={styles.linkPreviewTexts}>
                        {preview.site_name && (
                            <span className={styles.linkPreviewSite}>{preview.site_name}</span>
                        )}
                        <span className={styles.linkPreviewTitle}>
                            {preview.title ?? preview.url}
                        </span>
                        {preview.description && (
                            <span className={styles.linkPreviewDesc}>{preview.description}</span>
                        )}
                    </div>
                    {preview.image_url && (
                        <img
                            src={preview.image_url}
                            alt=""
                            className={styles.linkPreviewThumb}
                            loading="lazy"
                        />
                    )}
                    <button
                        type="button"
                        className={styles.contextClose}
                        aria-label="Скрыть превью ссылки"
                        onClick={() => setDismissedUrl(activeUrl)}
                    >
                        <Icon name="close" size={18} color="inherit" />
                    </button>
                </div>
            )}

            {context && (
                <div className={styles.context}>
                    {editing && <Icon name="edit" size={18} color="var(--color-accent)" />}
                    <span className={styles.contextTitle}>{context.title}</span>
                    <span className={styles.contextText}>{context.text}</span>
                    <button
                        type="button"
                        className={styles.contextClose}
                        aria-label={editing ? 'Отменить редактирование' : 'Отменить ответ'}
                        onClick={onCancel}
                    >
                        <Icon name="close" size={18} color="inherit" />
                    </button>
                </div>
            )}

            <div className={styles.row}>
                <RichEditor
                    ref={editorRef}
                    className={styles.field}
                    initial={initial}
                    placeholder="Введите сообщение"
                    aria-label="Сообщение"
                    maxLength={MAX_MESSAGE_LENGTH}
                    onChange={handleChange}
                    onCaretChange={handleCaretChange}
                    onSpecialKeyDown={handleSpecialKeyDown}
                    onSubmit={submit}
                    onEscape={() => {
                        if (mention) {
                            setMention(null)
                            return
                        }
                        if (editing || replyTo) {
                            onCancel()
                        }
                    }}
                />
                {/* Esc закрывает панель и возвращает фокус в поле */}
                <div
                    ref={emojiRef}
                    className={styles.emoji}
                    onKeyDown={(event) => {
                        if (event.key === 'Escape' && emojiOpen) {
                            event.stopPropagation()
                            editorRef.current?.focus()
                        }
                    }}
                >
                    {emojiOpen && (
                        <div className={styles.emojiPanel}>
                            <EmojiPicker
                                onPick={insertEmoji}
                                favorites={favorites.list}
                                onToggleFavorite={favorites.toggle}
                            />
                        </div>
                    )}
                    <IconButton
                        icon="emoji"
                        aria-label="Эмодзи"
                        aria-haspopup="dialog"
                        aria-expanded={emojiOpen}
                        isActive={emojiOpen}
                        onClick={() => setEmojiOpen((open) => !open)}
                    />
                </div>
                <div ref={sendWrapRef} className={styles.sendWrap}>
                    {silentMenuOpen && !editing && canSubmit && (
                        <div
                            role="menu"
                            aria-label="Способ отправки"
                            className={styles.silentMenu}
                        >
                            <button
                                type="button"
                                role="menuitem"
                                className={styles.silentItem}
                                onClick={() => submit(true)}
                            >
                                <Icon name="notif-off" size={16} color="currentColor" />
                                <span>Отправить без звука</span>
                            </button>
                        </div>
                    )}
                    <IconButton
                        icon={editing ? 'check' : 'send'}
                        aria-label={editing ? 'Сохранить' : 'Отправить'}
                        className={styles.send}
                        disabled={!canSubmit}
                        onClick={() => {
                            if (suppressClickRef.current) {
                                suppressClickRef.current = false
                                return
                            }
                            submit(false)
                        }}
                        onContextMenu={(event) => {
                            if (editing || !canSubmit) {
                                return
                            }
                            event.preventDefault()
                            setSilentMenuOpen((open) => !open)
                        }}
                        onPointerDown={(event) => {
                            if (editing || !canSubmit || event.pointerType !== 'touch') {
                                return
                            }
                            clearSendLongPress()
                            sendPressTimerRef.current = setTimeout(() => {
                                sendPressTimerRef.current = null
                                suppressClickRef.current = true
                                setSilentMenuOpen(true)
                            }, SEND_LONG_PRESS_MS)
                        }}
                        onPointerUp={clearSendLongPress}
                        onPointerCancel={clearSendLongPress}
                    />
                </div>
            </div>
        </div>
    )
}
