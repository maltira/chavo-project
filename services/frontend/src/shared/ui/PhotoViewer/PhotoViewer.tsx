import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'

import { ConfirmModal } from '../ConfirmModal'
import { Icon } from '../Icon'
import { useModalLayer } from '../Modal/useModalLayer'
import { largeAvatarUrl } from './largeUrl'
import styles from './PhotoViewer.module.scss'

// фото в просмотре: src - адрес маленькой аватарки, крупная лежит рядом и подменяет её, как только загрузится
export interface ViewerPhoto {
    id: string
    src: string
}

interface Props {
    // история: главное (текущее) фото первым; хотя бы одно
    photos: ViewerPhoto[]
    // подпись в углу, подпись для скринридера и имя скачанного файла
    title?: string
    onClose: () => void
    // сделать фото главным и удалить его: только у того, кто вправе менять историю
    onSelect?: (photo: ViewerPhoto) => void
    onDelete?: (photo: ViewerPhoto) => void
    busy?: boolean
}

// просмотр фото на весь экран: затемнённый фон, справа сверху «Удалить», «Скачать» и крестик, стрелки и
// счётчик «2 из 5» для истории, «Сделать главной» для не главного фото; закрывается крестиком, Esc и кликом
// по фону
export function PhotoViewer({ photos, title, onClose, onSelect, onDelete, busy = false }: Props) {
    // показанное фото помним по id: после «сделать главной» и удаления список перестраивается, а просмотр
    // остаётся на этом же фото (или на соседнем, если его удалили)
    const [at, setAt] = useState({ id: photos[0]?.id ?? '', index: 0 })
    const found = photos.findIndex((p) => p.id === at.id)
    const index = found >= 0 ? found : Math.min(at.index, photos.length - 1)
    const photo = photos[index]
    const [confirming, setConfirming] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)

    // пока открыто подтверждение, Esc и клик по фону относятся к нему, а не к просмотру
    useModalLayer(!confirming, onClose, rootRef)

    const go = (to: number) => {
        const next = (to + photos.length) % photos.length
        const target = photos[next]
        if (target) {
            setAt({ id: target.id, index: next })
        }
    }

    // стрелки с клавиатуры листают историю
    useEffect(() => {
        if (confirming || photos.length < 2) {
            return
        }
        const onKey = (event: KeyboardEvent) => {
            if (event.key === 'ArrowLeft') {
                go(index + 1)
            } else if (event.key === 'ArrowRight') {
                go(index - 1)
            }
        }
        document.addEventListener('keydown', onKey)
        return () => document.removeEventListener('keydown', onKey)
        // eslint-disable-next-line react-hooks/exhaustive-deps -- go зависит от тех же photos и index
    }, [confirming, photos, index])

    if (!photo) {
        return null
    }

    function download(src: string) {
        const link = document.createElement('a')
        link.href = src
        link.download = `${title?.trim() || 'photo'}.jpg`
        document.body.appendChild(link)
        link.click()
        link.remove()
    }

    const many = photos.length > 1

    return createPortal(
        <div
            ref={rootRef}
            role="dialog"
            aria-modal="true"
            aria-label={title ? `Фото: ${title}` : 'Фото'}
            tabIndex={-1}
            className={styles.backdrop}
            onMouseDown={(event) => {
                // клик именно по фону, а не по фото и кнопкам
                if (event.target === event.currentTarget) {
                    onClose()
                }
            }}
        >
            {title && <p className={styles.title}>{title}</p>}
            <Slide key={photo.src} photo={photo}>
                {(shownSrc) => (
                    <div className={styles.tools}>
                        {onDelete && (
                            <button
                                type="button"
                                className={styles.tool}
                                aria-label="Удалить фото"
                                onClick={() => setConfirming(true)}
                            >
                                <Icon name="trash" size={22} color="inherit" />
                            </button>
                        )}
                        <button
                            type="button"
                            className={styles.tool}
                            aria-label="Скачать"
                            onClick={() => download(shownSrc)}
                        >
                            <Icon name="download" size={22} color="inherit" />
                        </button>
                        <button
                            type="button"
                            className={styles.tool}
                            aria-label="Закрыть"
                            onClick={onClose}
                        >
                            <Icon name="close" size={22} color="inherit" />
                        </button>
                    </div>
                )}
            </Slide>

            {many && (
                <>
                    <button
                        type="button"
                        className={`${styles.tool} ${styles.prev}`}
                        aria-label="Следующее, более раннее фото"
                        onClick={() => go(index + 1)}
                    >
                        <Icon name="arrow-left" size={22} color="inherit" />
                    </button>
                    <button
                        type="button"
                        className={`${styles.tool} ${styles.next}`}
                        aria-label="Предыдущее, более новое фото"
                        onClick={() => go(index - 1)}
                    >
                        <Icon name="arrow-right" size={22} color="inherit" />
                    </button>
                </>
            )}

            <div className={styles.footer}>
                {many && (
                    <span className={styles.counter} aria-live="polite">
                        {index + 1} из {photos.length}
                    </span>
                )}
                {onSelect && index > 0 && (
                    <button
                        type="button"
                        className={styles.makeMain}
                        disabled={busy}
                        onClick={() => onSelect(photo)}
                    >
                        Сделать главной
                    </button>
                )}
            </div>

            {onDelete && (
                <ConfirmModal
                    open={confirming}
                    onClose={() => setConfirming(false)}
                    onConfirm={() => {
                        setConfirming(false)
                        onDelete(photo)
                    }}
                    confirmLabel="Удалить"
                    danger
                    pending={busy}
                >
                    Фото будет удалено
                </ConfirmModal>
            )}
        </div>,
        document.body,
    )
}

interface SlideProps {
    photo: ViewerPhoto
    // кнопки справа сверху получают адрес показанного файла (для «Скачать»)
    children: (shownSrc: string) => ReactNode
}

// одно фото: крупная версия поверх маленькой; если крупной нет (старая аватарка) - остаётся маленькая
function Slide({ photo, children }: SlideProps) {
    const large = largeAvatarUrl(photo.src)
    const [shown, setShown] = useState(large)
    const [loaded, setLoaded] = useState(false)

    return (
        <>
            {children(shown)}
            {/* маленькая версия видна сразу и держит место, пока грузится крупная */}
            <div className={styles.frame} style={{ backgroundImage: `url("${photo.src}")` }}>
                <img
                    src={shown}
                    alt=""
                    draggable={false}
                    className={styles.photo}
                    data-loaded={loaded}
                    onLoad={() => setLoaded(true)}
                    onError={() => {
                        if (shown !== photo.src) {
                            setShown(photo.src)
                        }
                    }}
                />
            </div>
        </>
    )
}
