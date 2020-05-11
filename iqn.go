package main

import (
	"context"

	"github.com/whywaita/go-os-brick/osbrick"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

func (a *agent) GetISCSIQualifiedName(ctx context.Context, req *pb.GetISCSIQualifiedNameRequest) (*pb.GetISCSIQualifiedNameResponse, error) {
	iqn, err := osbrick.GetIQN(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get iqn: %+v", err)
	}

	return &pb.GetISCSIQualifiedNameResponse{
		Iqn: iqn,
	}, nil
}
