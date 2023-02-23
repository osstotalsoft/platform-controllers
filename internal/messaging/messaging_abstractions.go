package messaging

import (
	"context"
	"os"
	"strconv"

	rusi "totalsoft.ro/platform-controllers/internal/messaging/rusi"
)

type MessagingPublisher func(ctx context.Context, topic string, data interface{}, platform string) error

var (
	EnvRusiEnabled = "RUSI_ENABLED"
)

func NilMessagingPublisher(ctx context.Context, topic string, data interface{}, platform string) error {
	return nil
}

func DefaultMessagingPublisher() MessagingPublisher {
	if enabled, err := strconv.ParseBool(os.Getenv(EnvRusiEnabled)); err == nil && enabled {
		return rusi.Publish
	}
	return NilMessagingPublisher
}
