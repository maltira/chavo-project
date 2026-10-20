// тело загрузки фото: файл в поле file; имя файла сервер не использует
export function photoForm(photo: Blob): FormData {
    const form = new FormData()
    form.append('file', photo, 'photo.jpg')
    return form
}
