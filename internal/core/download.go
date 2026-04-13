package core

import "time"

// Download represents a downloaded audio file.
type Download struct {
	ID           int
	VideoID      string
	Title        string
	Channel      string
	FilePath     string
	FileSize     int64
	DownloadedAt time.Time
}
