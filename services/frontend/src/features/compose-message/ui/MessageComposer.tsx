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
    conversationId: string
    myId: string | undefined
    replyTo?: ReplyTarget | null
    // редактируемое сообщение; при смене родитель пересоздаёт компонент через key
    editing?: Message | null
    onCancel: () => void
}

// предел высоты поля: дальше текст прокручивается внутри
const MAX_HEIGHT = 160

// поле ввода сообщения: отправка, ответ и редактирование; Enter - отправить, Shift+Enter - перенос
export function MessageComposer({ conversationId, myId, replyTo, editing, onCancel }: Props) {
    const [text, setText] = useState(editing?.content ?? '')
    const fieldRef = useRef<HTMLTextAreaElement>(null)
    const send = useSendMessage(myId)

    // при редактировании поле открывается с текстом: подгоняем высоту и ставим курсор в конец,
    // иначе autoFocus оставил бы его в начале
    const editingId = editing?.id
    useEffect(() => {
        resize()
        const field = fieldRef.current
        if (editingId && field) {
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
            // scrollHeight не включает рамку, а высота поля считается вместе с ней (border-box):
            // без этой добавки внутри остаётся лишний пиксель и появляется полоса прокрутки
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
            conversation_id: conversationId,
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
                    // фокус при открытии чата, ответе и редактировании
                    autoFocus
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
