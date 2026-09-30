package utils

import "go.uber.org/zap"

// SafeGo запускает fn в горутине с recover, логируя панику
func SafeGo(log *zap.Logger, name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic in goroutine",
					zap.String("name", name),
					zap.Any("recover", r),
				)
			}
		}()
		fn()
	}()
}
