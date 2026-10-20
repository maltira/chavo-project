// Package media - пути загруженных файлов (аватарки пользователей и групп): единственный источник истины
// о формате. Путь выдаёт Gateway после загрузки, user-service и conversation-service принимают только его
//
// путь - /media/avatars/<32 hex>.jpg: случайное имя (128 бит), поэтому его нельзя угадать,
// а файл по нему - всегда JPEG, который перекодировал сервер
package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

const (
	// URLPrefix - начало публичного пути; по нему nginx отдаёт файлы через Gateway
	URLPrefix = "/media/"
	// AvatarsDir - каталог аватарок: и префикс ключа в хранилище, и часть пути
	AvatarsDir = "avatars/"
	// AvatarExt - аватарка всегда перекодируется в JPEG
	AvatarExt = ".jpg"
)

var (
	ErrInvalidAvatarURL = errors.New("invalid avatar url")

	avatarName = regexp.MustCompile(`^[0-9a-f]{32}\.jpg$`)
)

// NewAvatarName - случайное имя файла аватарки: <32 hex>.jpg
func NewAvatarName() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + AvatarExt, nil
}

// ValidAvatarName - имя файла аватарки из пути или запроса: только формат, который выдаёт NewAvatarName
func ValidAvatarName(name string) bool {
	return avatarName.MatchString(name)
}

// AvatarKey - ключ объекта в хранилище по имени файла
func AvatarKey(name string) string { return AvatarsDir + name }

// AvatarURL - публичный путь аватарки по имени файла
func AvatarURL(name string) string { return URLPrefix + AvatarsDir + name }

// AvatarNameFromURL - имя файла из пути, который выдал сервер; false - путь не наш
// (например, внешний URL, записанный до появления загрузки)
func AvatarNameFromURL(url string) (string, bool) {
	name, ok := strings.CutPrefix(url, URLPrefix+AvatarsDir)
	if !ok || !ValidAvatarName(name) {
		return "", false
	}
	return name, true
}

// NormalizeAvatarURL проверяет новое значение avatar_url перед записью: пустая строка - аватарку убрали
// (nil, запишется NULL), иначе принимается только путь, который выдал сервер
func NormalizeAvatarURL(url string) (*string, error) {
	if url == "" {
		return nil, nil
	}
	if _, ok := AvatarNameFromURL(url); !ok {
		return nil, ErrInvalidAvatarURL
	}
	return &url, nil
}
