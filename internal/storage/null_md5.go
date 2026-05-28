package storage

import (
	"context"
	"lfs/internal/interfaces"
)

var _ interfaces.MD5Cache = (*NullMD5Cache)(nil)
var _ interfaces.MD5Calculator = (*NullMD5Calculator)(nil)

// NullMD5Cache is a no-op implementation of MD5Cache.
type NullMD5Cache struct{}

// NewNullMD5Cache creates and returns a new NullMD5Cache instance.
func NewNullMD5Cache() *NullMD5Cache {
	return &NullMD5Cache{}
}

func (c *NullMD5Cache) GetMD5(filePath, fileName string, size int64, modTime int64) (string, bool) {
	return "", false // Always say MD5 does not exist in cache
}

func (c *NullMD5Cache) SetMD5(filePath, fileName, md5 string, size int64, modTime int64) error {
	return nil
}

func (c *NullMD5Cache) SetCalculating(filePath, fileName string, size int64, modTime int64) error {
	return nil
}

func (c *NullMD5Cache) UpdateProgress(filePath string, progress float64) error {
	return nil
}

func (c *NullMD5Cache) SetError(filePath string, err error) error {
	return nil
}

func (c *NullMD5Cache) GetProgress(filePath string) (float64, bool, string) {
	return 0.0, false, ""
}

// NullMD5Calculator is a no-op implementation of MD5Calculator.
type NullMD5Calculator struct{}

// NewNullMD5Calculator creates and returns a new NullMD5Calculator instance.
func NewNullMD5Calculator() *NullMD5Calculator {
	return &NullMD5Calculator{}
}

func (c *NullMD5Calculator) GetMD5(ctx context.Context, filePath string) (string, error) {
	return "", nil
}

func (c *NullMD5Calculator) GetMD5Progress(filePath string) (float64, bool, string) {
	return 0.0, false, ""
}

func (c *NullMD5Calculator) CalculateMD5(ctx context.Context, filePath string, progressCallback func(float64)) (string, error) {
	if progressCallback != nil {
		progressCallback(0.0)
	}
	return "", nil
}
