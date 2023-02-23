package rusi

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "totalsoft.ro/platform-controllers/internal/messaging/rusi/v1"
)

var (
	conn *grpc.ClientConn
)

func newRusiClient(ctx context.Context) (cl v1.RusiClient, err error) {
	port := os.Getenv("RUSI_GRPC_PORT")
	address := fmt.Sprintf("localhost:%s", port)
	retryPolicy := `{
		"methodConfig": [{
		  "name": [{"service": "rusi.proto.runtime.v1.Rusi"}],
		  "waitForReady": true,
		  "retryPolicy": {
			  "MaxAttempts": 4,
			  "InitialBackoff": ".01s",
			  "MaxBackoff": ".01s",
			  "BackoffMultiplier": 1.0,
			  "RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		  }]}`

	if conn == nil {
		ctx := context.TODO()
		conn, err = grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(retryPolicy))
		if err != nil {
			return nil, err
		}
	}
	return v1.NewRusiClient(conn), nil
}

func marshal(data interface{}) ([]byte, error) {
	return jsoniter.Marshal(data)
}

func Publish(ctx context.Context, topic string, data interface{}, platform string) error {
	var (
		correlationId  string = "nbb-correlationId"
		source         string = "nbb-source"
		rusiPubSubName string = fmt.Sprintf("%s-pubsub", platform)
	)
	rc, err := newRusiClient(ctx)
	if err != nil {
		return err
	}

	jsonData, err := marshal(data)
	if err != nil {
		return err
	}

	metadata := map[string]string{correlationId: uuid.New().String(), source: "platform-controllers"}

	var pr = v1.PublishRequest{
		PubsubName: rusiPubSubName,
		Topic:      topic,
		Data:       jsonData,
		Metadata:   metadata,
	}
	_, err = rc.Publish(ctx, &pr)
	return err
}
