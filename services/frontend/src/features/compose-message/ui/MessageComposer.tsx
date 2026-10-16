import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'

import { MAX_MESSAGE_LENGTH } from '@/entities/message'
import type { Message } from '@/entities/message'
import { Icon, IconButton } from '@/shared/ui'

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

// предел высоты поля: дальше текст прокручивается внутри
const MAX_HEIGHT = 160

// поле ввода сообщения: отправка, ответ и редактирование; Enter - отправить, Shift+Enter - перенос
export function MessageComposer({
    conversationId,
    recipientId,
    myId,
    onSent,
    replyTo,
    editing,
    onCancel,
}: Props) {
    const [text, setText] = useState(editing?.content ?? '')
    const fieldRef = useRef<HTMLTextAreaElement>(null)
    const send = useSendMessage(myId, onSent)

    // при открытии чата фокуса нет - пользователь сам нажмёт на поле;
    // при редактировании - фокус и курсор в конце текста, высота по тексту
    const editingId = editing?.id
    useEffect(() => {
        resize()
        const field = fieldRef.current
        if (editingId && field) {
            field.focus()
            const end = field.value.length
            field.setSelectionRange(end, end)
            // длинный текст не должен оставаться прокрученным к началу
            field.scrollTop = field.scrollHeight
        }
    }, [editingId])

    // выбрали сообщение для ответа - курсор сразу в поле, можно печатать
    const replyId = replyTo?.id
    useEffect(() => {
        if (replyId) {
            fieldRef.current?.focus()
        }
    }, [replyId])
    const edit = useEditMessage()

    const content = text.trim()
    const canSubmit = content.length > 0 && content !== (editing?.content ?? '').trim()

    function resize() {
        const field = fieldRef.current
        if (field) {
            field.style.height = 'auto'
            const border = field.offsetHeight - field.clientHeight
            const wanted = field.scrollHeight + border
            field.style.height = `${Math.min(wanted, MAX_HEIGHT)}px`
            // полоса прокрутки - только когда высота дошла до предела
            field.style.overflowY = wanted > MAX_HEIGHT ? 'auto' : 'hidden'
        }
    }

    function submit() {
        if (!canSubmit) {
            return
        }
        if (editing) {
            edit.mutate({ id: editing.id, content })
            onCancel()
            return
        }
        send.mutate({
            ...(conversationId
                ? { conversation_id: conversationId }
                : { recipient_id: recipientId }),
            content,
            reply_to_message_id: replyTo?.id ?? null,
        })
        setText('')
        // высота поля возвращается к одной строке после очистки
        requestAnimationFrame(resize)
        onCancel()
    }

    function onKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
        if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
            event.preventDefault()
            submit()
        }
        if (event.key === 'Escape' && (editing || replyTo)) {
            onCancel()
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
                <textarea
                    ref={fieldRef}
                    className={styles.field}
                    rows={1}
                    placeholder="Введите сообщение"
                    aria-label="Сообщение"
                    value={text}
                    maxLength={MAX_MESSAGE_LENGTH}
                    onChange={(event) => {
                        setText(event.target.value)
                        resize()
                    }}
                    onKeyDown={onKeyDown}
                />
                <IconButton
                    icon={editing ? 'check' : 'send'}
                    aria-label={editing ? 'Сохранить' : 'Отправить'}
                    className={styles.send}
                    disabled={!canSubmit}
                    onClick={submit}
                />
            </div>
        </div>
    )
}
