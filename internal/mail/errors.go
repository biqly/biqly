package mail

import "errors"

// ErrEmailBlocked is returned when the destination address is on the
// transactional block list (bounced, marked-as-spam, manually blocked).
var ErrEmailBlocked = errors.New("email address is blocked")

// ErrEmailRateLimited is returned when the destination address has already
// received the configured maximum number of emails for the current day.
var ErrEmailRateLimited = errors.New("daily email rate limit exceeded for recipient")
