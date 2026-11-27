import { useCallback, useEffect, useRef, useState } from 'react'

import { sendTyping, TYPING_SEND_INTERVAL_MS } from '@/entities/conversation'
import { getDraft, MAX_MESSAGE_LENGTH, useDrafts } from '@/entities/message'
import type { Message } from '@/entities/message'
import { useFavoriteEmojis } from '@/entities/user'
import { sameValue, trimValue, useDismiss } from '@/shared/lib'
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
    // сервер принял сообщение (для нового чата - в нём уже есть conversation_id)
    onSent?: (message: Message) => void
    replyTo?: ReplyTarget | null
    // редактируемое сообщение; при смене родитель пересоздаёт компонент через key
    editing?: Message | null
    onCancel: () => void
}

const EMPTY: RichValue = { text: '', entities: [] }
const SEND_LONG_PRESS_MS = 450

// текст и форматирование сообщения, которое правят
function valueOf(message: Message | null | undefined): RichValue {
    return { text: message?.content ?? '', entities: message?.entities ?? [] }
}

// сохранённый черновик чата
function draftValue(chatKey: string | undefined): RichValue {
    const draft = getDraft(chatKey)
    return draft ? { text: draft.text, entities: draft.entities } : EMPTY
}

// поле ввода сообщения: отправка, ответ и редактирование; жирный, курсив и подчёркивание (панель над
// выделением и Ctrl/Cmd + B / I / U); Enter - отправить, Shift+Enter - перенос
export function MessageComposer({
    conversationId,
    recipientId,
    myId,
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
    const closeEmoji = useCallback(() => setEmojiOpen(false), [])
    useDismiss(emojiOpen, emojiRef, closeEmoji)
    const favorites = useFavoriteEmojis(emojiOpen)

    // меню «Отправить без звука» по правому клику или долгому нажатию на кнопку отправки
    const [silentMenuOpen, setSilentMenuOpen] = useState(false)
    const sendWrapRef = useRef<HTMLDivElement>(null)
    const closeSilentMenu = useCallback(() => setSilentMenuOpen(false), [])
    useDismiss(silentMenuOpen, sendWrapRef, closeSilentMenu)
    const sendPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const suppressClickRef = useRef(false)
    const lastTypingRef = useRef(0)

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
    const canSubmit =
        trimmed.text.length > 0 && (!editing || !sameValue(trimmed, trimValue(valueOf(editing))))

    function submit(silent = false) {
        if (!canSubmit) {
            return
        }
        setSilentMenuOpen(false)
        if (editing) {
            edit.mutate({ id: editing.id, content: trimmed.text, entities: trimmed.entities })
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
            reply_to_message_id: replyTo?.id ?? null,
        })
        // поле и черновик очищаются, форматирование сбрасывается
        if (draftKey) {
            clearDraft(draftKey)
        }
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
                    onSubmit={submit}
                    onEscape={() => {
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
