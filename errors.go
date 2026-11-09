package pixellab

import "errors"

// Sentinel errors of the plugin, matched with errors.Is.
var (
	ErrInvalidSetting    = errors.New("invalid pixellab setting")
	ErrNoAccount         = errors.New("no pixellab account")
	ErrInvalidAccount    = errors.New("invalid pixellab account")
	ErrInvalidOption     = errors.New("invalid pixellab option")
	ErrInvalidAttachment = errors.New("invalid pixellab attachment")
	ErrNoOutbox          = errors.New("no outbox for pixellab files")
	ErrSubscriptionDry   = errors.New("pixellab subscription exhausted")
	ErrServer            = errors.New("pixellab server error")
	ErrNoDelivery        = errors.New("no pixellab account delivered the image")
)
