// фото перед загрузкой: проверка, кроп, кодирование и черновик выбранного фото
export {
    AVATAR_MIN_SIDE,
    AVATAR_UPLOAD_SIZE,
    checkImageFile,
    clampCrop,
    coverScale,
    encodeSquare,
    IMAGE_ACCEPT,
    isTooSmall,
    loadImage,
    MAX_ZOOM,
    maxZoomFor,
    MIN_ZOOM,
    sourceRect,
} from './image'
export type { CropState, LoadedImage } from './image'
export { usePhotoDraft } from './usePhotoDraft'
