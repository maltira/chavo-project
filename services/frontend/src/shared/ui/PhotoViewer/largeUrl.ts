// крупная версия загруженной аватарки: рядом с /media/avatars/<имя>.jpg лежит <имя>_l.jpg (до 1024 px);
// чужие и внешние адреса остаются как есть, у старых аватарок крупной версии нет (фото откроется маленьким)
const OUR_AVATAR = /^\/media\/avatars\/[0-9a-f]{32}\.jpg$/

export function largeAvatarUrl(url: string): string {
    return OUR_AVATAR.test(url) ? url.replace(/\.jpg$/, '_l.jpg') : url
}
