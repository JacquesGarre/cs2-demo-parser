package entities

import "time"

type Demo struct {
	ID          string
	FileName    string
	StoragePath string
	UploadedAt  time.Time
}
