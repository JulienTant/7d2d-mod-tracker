package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func newAppLogger() (*log.Logger, string, io.Closer, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds), "", nil, err
	}
	logDir := filepath.Join(cacheDir, "7d2d-mod-tracker", "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds), logDir, nil, err
	}
	filename := fmt.Sprintf("7d2d-mod-tracker-%s.log", time.Now().Format("2006-01-02"))
	file, err := os.OpenFile(filepath.Join(logDir, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds), logDir, nil, err
	}
	logger := log.New(io.MultiWriter(os.Stderr, file), "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return logger, logDir, file, nil
}
