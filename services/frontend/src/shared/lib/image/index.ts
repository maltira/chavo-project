// фото перед загрузкой: проверка, кроп, кодирование и черновик выбранного фото
export {
    AVATAR_UPLOAD_SIZE,
    checkImageFile,
    clampCrop,
    coverScale,
    encodeSquare,
    IMAGE_ACCEPT,
    loadImage,
    MAX_ZOOM,
    MIN_ZOOM,
    sourceRect,
} from './image'
export type { CropState, LoadedImage } from './image'
export { usePhotoDraft } from './usePhotoDraft'
