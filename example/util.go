package example

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	pb "github.com/lovi-cloud/teleskop/protoc/agent"
)

func SetupClient(ctx context.Context, addr string) (pb.AgentClient, error) {
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithBlock(),
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                1000 * time.Millisecond,
			Timeout:             3000 * time.Millisecond,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to teleskop agent: %w", err)
	}

	return pb.NewAgentClient(conn), nil
}
