import { useRef, useState } from 'react'
import { createPortal } from 'react-dom'

import { ConfirmModal } from '../ConfirmModal'
import { Icon } from '../Icon'
import { useModalLayer } from '../Modal/useModalLayer'
import { largeAvatarUrl } from './largeUrl'
import styles from './PhotoViewer.module.scss'

interface Props {
    // адрес маленькой аватарки; крупная лежит рядом и подменяет её, как только загрузится
    src: string
    // подпись в углу, подпись для скринридера и имя скачанного файла
    title?: string
    onClose: () => void
    // удалить фото (с подтверждением): только у того, кто вправе его менять
    onDelete?: () => void
    deleting?: boolean
}

// просмотр фото на весь экран: затемнённый фон, справа сверху «Удалить», «Скачать» и крестик;
// закрывается крестиком, Esc и кликом по фону
export function PhotoViewer({ src, title, onClose, onDelete, deleting = false }: Props) {
    const large = largeAvatarUrl(src)
    // что показано сейчас: крупная версия, а если её нет (старая аватарка) - маленькая
    const [shown, setShown] = useState(large)
    const [loaded, setLoaded] = useState(false)
    const [confirming, setConfirming] = useState(false)
    const rootRef = useRef<HTMLDivElement>(null)

    // пока открыто подтверждение, Esc и клик по фону относятся к нему, а не к просмотру
    useModalLayer(!confirming, onClose, rootRef)

    function download() {
        const link = document.createElement('a')
        link.href = shown
        link.download = `${title?.trim() || 'photo'}.jpg`
        document.body.appendChild(link)
        link.click()
        link.remove()
    }

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
                    onClick={download}
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

            {/* маленькая версия видна сразу и держит место, пока грузится крупная */}
            <div className={styles.frame} style={{ backgroundImage: `url("${src}")` }}>
                <img
                    src={shown}
                    alt={title ?? ''}
                    draggable={false}
                    className={styles.photo}
                    data-loaded={loaded}
                    onLoad={() => setLoaded(true)}
                    onError={() => {
                        if (shown !== src) {
                            setShown(src)
                        }
                    }}
                />
            </div>

            {onDelete && (
                <ConfirmModal
                    open={confirming}
                    onClose={() => setConfirming(false)}
                    onConfirm={() => {
                        setConfirming(false)
                        onDelete()
                    }}
                    confirmLabel="Удалить"
                    danger
                    pending={deleting}
                >
                    Фото будет удалено
                </ConfirmModal>
            )}
        </div>,
        document.body,
    )
}
