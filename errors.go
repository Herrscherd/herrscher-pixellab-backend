package pixellab

import (
	"context"
	"errors"
)

var (
	// ErrInvalidSetting is returned when a configured setting does not parse or is out of range.
	ErrInvalidSetting = errors.New("invalid pixellab setting")
	// ErrNoAccount is returned when no account source resolves to an account.
	ErrNoAccount = errors.New("no pixellab account")
	// ErrInvalidAccount is returned when an account line or the Aseprite manifest is malformed.
	ErrInvalidAccount = errors.New("invalid pixellab account")
	// ErrInvalidOption is returned when the prompt carries an unknown or out of range option.
	ErrInvalidOption = errors.New("invalid pixellab option")
	// ErrInvalidAttachment is returned when an attachment is not a usable PNG for the mode.
	ErrInvalidAttachment = errors.New("invalid pixellab attachment")
	// ErrNoOutbox is returned when the host gives a prompt without an outbox.
	ErrNoOutbox = errors.New("no outbox for pixellab files")
	// ErrSubscriptionDry is returned when an account's subscription has no quota left.
	ErrSubscriptionDry = errors.New("pixellab subscription exhausted")
	// ErrServer is returned when PixelLab answers with an error or an unusable frame.
	ErrServer = errors.New("pixellab server error")
	// ErrNoDelivery is returned when every account failed to deliver an image.
	ErrNoDelivery = errors.New("no pixellab account delivered the image")
	// ErrTooManyFrames is returned when one request receives more frames than a reply may carry.
	ErrTooManyFrames = errors.New("pixellab sent too many frames")
	// ErrMessageTooLarge is returned when one frame exceeds the size a reply may carry.
	ErrMessageTooLarge = errors.New("pixellab frame too large")
)

func redactionSentinels() []error {
	return []error{
		ErrInvalidSetting,
		ErrNoAccount,
		ErrInvalidAccount,
		ErrInvalidOption,
		ErrInvalidAttachment,
		ErrNoOutbox,
		ErrSubscriptionDry,
		ErrServer,
		ErrNoDelivery,
		ErrTooManyFrames,
		ErrMessageTooLarge,
		context.Canceled,
		context.DeadlineExceeded,
	}
}
