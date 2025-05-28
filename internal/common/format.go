package common

import "fmt"

func PrettyFormatSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(size)/(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(size)/(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func PrettyFormatSpeed(speed int) string {
	// Convert speed to a human-readable format
	speedFloat := float64(speed)
	switch {
	case speed >= 1<<30:
		return fmt.Sprintf("%.2f GB/s", speedFloat/(1<<30))
	case speed >= 1<<20:
		return fmt.Sprintf("%.2f MB/s", speedFloat/(1<<20))
	case speed >= 1<<10:
		return fmt.Sprintf("%.2f KB/s", speedFloat/(1<<10))
	default:
		return fmt.Sprintf("%d B/s", speed)
	}
}

func PrettyFormatDuration(duration int64, speed int32) string {
	// Convert duration to a human-readable format
	if speed == 0 {
		return "N/A"
	}
	seconds := duration / int64(speed)
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	seconds = seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
