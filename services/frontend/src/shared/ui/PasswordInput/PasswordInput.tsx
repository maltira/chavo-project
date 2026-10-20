import { useState } from 'react'
import type { ComponentProps } from 'react'

import { Input } from '../Input'

type Props = Omit<
    ComponentProps<typeof Input>,
    'type' | 'rightIcon' | 'rightIconLabel' | 'onRightIconClick'
>

// поле пароля с глазиком: показать или скрыть введённое
export function PasswordInput(props: Props) {
    const [visible, setVisible] = useState(false)

    return (
        <Input
            {...props}
            type={visible ? 'text' : 'password'}
            rightIcon={visible ? 'eye-open' : 'eye-closed'}
            rightIconLabel={visible ? 'Скрыть пароль' : 'Показать пароль'}
            onRightIconClick={() => setVisible((value) => !value)}
        />
    )
}
