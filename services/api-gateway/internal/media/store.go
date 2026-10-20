// Package media - загруженные файлы (аватарки): проверка и перекодирование изображений
// и объектное хранилище S3 (MinIO). С хранилищем работает только Gateway: сервисы говорят лишь по gRPC
package media

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound - объекта нет в хранилище
var ErrNotFound = errors.New("media object not found")

// Store - объектное хранилище; ключ - путь объекта внутри бакета (например, avatars/<id>.jpg)
type Store interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Get отдаёт содержимое объекта; ErrNotFound, если его нет. Вызывающий закрывает Body
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete не считает ошибкой отсутствие объекта
	Delete(ctx context.Context, key string) error
}
