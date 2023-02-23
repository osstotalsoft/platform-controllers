package mock

import (
	"context"

	messaging "totalsoft.ro/platform-controllers/internal/messaging"
)

func MessagingPublisherMock(msgChan chan RcvMsg) messaging.MessagingPublisher {
	return func(ctx context.Context, topic string, data interface{}, platform string) error {
		go func() {
			msgChan <- RcvMsg{topic, data}
		}()
		return nil
	}

}

type RcvMsg struct {
	Topic string
	Data  interface{}
}
