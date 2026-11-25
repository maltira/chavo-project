import { useState } from 'react'

import { useUser } from '@/entities/user'
import { Button, Checkbox, Modal } from '@/shared/ui'

import styles from './PinConfirmModal.module.scss'

interface Props {
    mode: 'pin' | 'unpin'
    peerId: string | null
    // флажок «...также для собеседника»: при откреплении - только если закреп общий
    withPeerOption: boolean
    onClose: () => void
    // forBoth - флажок отмечен
    onConfirm: (forBoth: boolean) => void
}

const TEXT = {
    pin: {
        title: 'Закрепить сообщение',
        question: 'Закрепить это сообщение?',
        option: 'Также закрепить для',
        action: 'Закрепить',
    },
    unpin: {
        title: 'Открепить сообщение',
        question: 'Открепить это сообщение?',
        option: 'Открепить также для',
        action: 'Открепить',
    },
}

// подтверждение закрепа и открепления в личном чате; флажок (по умолчанию выключен) решает,
// касается ли действие собеседника
export function PinConfirmModal({ mode, peerId, withPeerOption, onClose, onConfirm }: Props) {
    const peer = useUser(withPeerOption ? peerId : null)
    const name = peer.data?.display_name ?? 'собеседника'
    const [forBoth, setForBoth] = useState(false)
    const text = TEXT[mode]

    return (
        <Modal open onClose={onClose} title={text.title} titleAlign="start" width={424}>
            <p className={styles.text}>{text.question}</p>
            {withPeerOption && (
                <Checkbox checked={forBoth} onChange={(event) => setForBoth(event.target.checked)}>
                    {text.option} {name}
                </Checkbox>
            )}
            <Button variant="secondary" onClick={() => onConfirm(withPeerOption && forBoth)}>
                {text.action}
            </Button>
        </Modal>
    )
}
