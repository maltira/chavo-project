import { useState } from 'react'
import type { ChangeEvent, ReactNode } from 'react'

import { checkImageFile, IMAGE_ACCEPT, notify } from '@/shared/lib'

import { ImageCropper } from '../ImageCropper'

interface Props {
    // готовое фото (квадрат JPEG после кропа)
    onPick: (photo: Blob) => void
    // кнопка или ссылка, которая открывает выбор файла
    children: (open: () => void) => ReactNode
}

// выбор фото с диска: проверка файла, затем окно кропа; наружу отдаётся уже готовый квадрат
export function PhotoPicker({ onPick, children }: Props) {
    // элемент поля через callback-ref: открыть диалог можно из обработчика, без чтения ref при рендере
    const [input, setInput] = useState<HTMLInputElement | null>(null)
    const [file, setFile] = useState<File | null>(null)

    function onChange(event: ChangeEvent<HTMLInputElement>) {
        const picked = event.target.files?.[0]
        // сброс, чтобы тот же файл можно было выбрать ещё раз
        event.target.value = ''
        if (!picked) {
            return
        }
        const error = checkImageFile(picked)
        if (error) {
            notify({ icon: 'warn', title: 'Это фото не подойдёт', description: error })
            return
        }
        setFile(picked)
    }

    return (
        <>
            {children(() => input?.click())}
            <input
                ref={setInput}
                type="file"
                accept={IMAGE_ACCEPT}
                hidden
                aria-hidden="true"
                tabIndex={-1}
                data-testid="photo-input"
                onChange={onChange}
            />
            <ImageCropper
                file={file}
                onCancel={() => setFile(null)}
                onConfirm={(photo) => {
                    setFile(null)
                    onPick(photo)
                }}
            />
        </>
    )
}
