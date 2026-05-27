package mail

import "time"

// DeviceLoginInfo carries the metadata rendered into a new-device-login email.
type DeviceLoginInfo struct {
	UserAgent  string
	IPAddress  string
	OccurredAt time.Time
}
