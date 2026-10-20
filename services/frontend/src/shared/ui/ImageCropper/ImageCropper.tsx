import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent, PointerEvent, WheelEvent } from 'react'

import {
    clampCrop,
    coverScale,
    encodeSquare,
    loadImage,
    MAX_ZOOM,
    MIN_ZOOM,
    notify,
    sourceRect,
} from '@/shared/lib'
import type { CropState, LoadedImage } from '@/shared/lib'

import { Button } from '../Button'
import { Icon } from '../Icon'
import { Loader } from '../Loader'
import { Modal } from '../Modal'
import styles from './ImageCropper.module.scss'

// сторона окна кропа на экране, px
const VIEWPORT = 280
// шаг сдвига стрелками и зума клавишами
const KEY_STEP = 12
const ZOOM_STEP = 0.1

interface Props {
    // выбранный файл; null - окно закрыто
    file: File | null
    onCancel: () => void
    // готовый квадрат в JPEG
    onConfirm: (blob: Blob) => void
}

// кроп фото под аватарку: круг, перетаскивание, зум колесом, ползунком и клавишами
export function ImageCropper({ file, onCancel, onConfirm }: Props) {
    // при закрытии содержимое остаётся прежним, пока играет анимация
    const [shown, setShown] = useState(file)
    if (file && file !== shown) {
        setShown(file)
    }

    return (
        <Modal
            open={file !== null}
            onClose={onCancel}
            title="Фото"
            headerStart={
                <button type="button" className={styles.back} aria-label="Назад" onClick={onCancel}>
                    <Icon name="arrow-left" size={24} color="inherit" />
                </button>
            }
        >
            {shown && (
                <CropperBody
                    key={shown.name + shown.size + shown.lastModified}
                    file={shown}
                    onCancel={onCancel}
                    onConfirm={onConfirm}
                />
            )}
        </Modal>
    )
}

interface BodyProps {
    file: File
    onCancel: () => void
    onConfirm: (blob: Blob) => void
}

function CropperBody({ file, onCancel, onConfirm }: BodyProps) {
    const [image, setImage] = useState<LoadedImage | null>(null)
    const [crop, setCrop] = useState<CropState>({ zoom: MIN_ZOOM, x: 0, y: 0 })
    const [saving, setSaving] = useState(false)
    const canvasRef = useRef<HTMLCanvasElement>(null)
    const dragRef = useRef<{ id: number; px: number; py: number; x: number; y: number } | null>(
        null,
    )

    // декодирование файла; не открылся - сообщаем и закрываем окно
    useEffect(() => {
        let alive = true
        let loaded: LoadedImage | null = null
        loadImage(file).then(
            (img) => {
                loaded = img
                if (alive) {
                    setImage(img)
                }
            },
            () => {
                if (alive) {
                    notify({
                        icon: 'warn',
                        title: 'Не удалось открыть фото',
                        description: 'Попробуйте другой файл',
                    })
                    onCancel()
                }
            },
        )
        return () => {
            alive = false
            const source = loaded?.source
            if (source && 'close' in source) {
                source.close()
            }
        }
    }, [file, onCancel])

    // превью рисуется той же функцией, что и итоговый кадр: на сервер уходит ровно то, что видно в круге
    useEffect(() => {
        const canvas = canvasRef.current
        const ctx = canvas?.getContext('2d')
        if (!canvas || !ctx || !image) {
            return
        }
        const ratio = window.devicePixelRatio || 1
        canvas.width = VIEWPORT * ratio
        canvas.height = VIEWPORT * ratio
        const rect = sourceRect(crop, image.width, image.height, VIEWPORT)
        ctx.imageSmoothingQuality = 'high'
        ctx.clearRect(0, 0, canvas.width, canvas.height)
        ctx.drawImage(
            image.source,
            rect.sx,
            rect.sy,
            rect.size,
            rect.size,
            0,
            0,
            canvas.width,
            canvas.height,
        )
    }, [image, crop])

    function update(next: CropState) {
        if (image) {
            setCrop(clampCrop(next, image.width, image.height, VIEWPORT))
        }
    }

    // зум вокруг центра окна: смещение растёт вместе с масштабом
    function zoomTo(zoom: number) {
        const z = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom))
        update({ zoom: z, x: (crop.x * z) / crop.zoom, y: (crop.y * z) / crop.zoom })
    }

    function onPointerDown(event: PointerEvent<HTMLDivElement>) {
        event.currentTarget.setPointerCapture?.(event.pointerId)
        dragRef.current = {
            id: event.pointerId,
            px: event.clientX,
            py: event.clientY,
            x: crop.x,
            y: crop.y,
        }
    }

    function onPointerMove(event: PointerEvent<HTMLDivElement>) {
        const drag = dragRef.current
        if (!drag || drag.id !== event.pointerId) {
            return
        }
        update({
            zoom: crop.zoom,
            x: drag.x + event.clientX - drag.px,
            y: drag.y + event.clientY - drag.py,
        })
    }

    function onPointerUp() {
        dragRef.current = null
    }

    function onWheel(event: WheelEvent<HTMLDivElement>) {
        zoomTo(crop.zoom * Math.exp(-event.deltaY * 0.0015))
    }

    function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
        const moves: Record<string, [number, number]> = {
            ArrowLeft: [KEY_STEP, 0],
            ArrowRight: [-KEY_STEP, 0],
            ArrowUp: [0, KEY_STEP],
            ArrowDown: [0, -KEY_STEP],
        }
        const move = moves[event.key]
        if (move) {
            event.preventDefault()
            update({ zoom: crop.zoom, x: crop.x + move[0], y: crop.y + move[1] })
        } else if (event.key === '+' || event.key === '=') {
            event.preventDefault()
            zoomTo(crop.zoom + ZOOM_STEP)
        } else if (event.key === '-') {
            event.preventDefault()
            zoomTo(crop.zoom - ZOOM_STEP)
        }
    }

    async function save() {
        if (!image) {
            return
        }
        setSaving(true)
        try {
            onConfirm(
                await encodeSquare(image, sourceRect(crop, image.width, image.height, VIEWPORT)),
            )
        } catch {
            notify({
                icon: 'warn',
                title: 'Не удалось обработать фото',
                description: 'Попробуйте другой файл',
            })
        } finally {
            setSaving(false)
        }
    }

    // насколько фото больше окна - для подсказки, можно ли его двигать
    const movable =
        !!image &&
        coverScale(image.width, image.height, VIEWPORT) *
            crop.zoom *
            Math.max(image.width, image.height) >
            VIEWPORT

    return (
        <div className={styles.body}>
            <div
                className={styles.viewport}
                style={{ width: VIEWPORT, height: VIEWPORT }}
                role="group"
                aria-label="Кадр фото: перетаскивайте фото, приближайте колесом мыши или клавишами + и −"
                tabIndex={0}
                onPointerDown={onPointerDown}
                onPointerMove={onPointerMove}
                onPointerUp={onPointerUp}
                onPointerCancel={onPointerUp}
                onWheel={onWheel}
                onKeyDown={onKeyDown}
                data-movable={movable}
            >
                {image ? (
                    <canvas ref={canvasRef} className={styles.canvas} aria-hidden="true" />
                ) : (
                    <span className={styles.loading}>
                        <Loader size={48} />
                    </span>
                )}
                <span className={styles.mask} aria-hidden="true" />
            </div>

            <label className={styles.zoom}>
                <Icon name="search" size={20} color="inherit" />
                <input
                    type="range"
                    aria-label="Масштаб"
                    min={MIN_ZOOM}
                    max={MAX_ZOOM}
                    step={0.01}
                    value={crop.zoom}
                    disabled={!image}
                    onChange={(event) => zoomTo(Number(event.target.value))}
                />
            </label>

            <Button disabled={!image || saving} onClick={() => void save()}>
                Сохранить
            </Button>
        </div>
    )
}
