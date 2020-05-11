package main

import (
	"context"

	"github.com/whywaita/go-os-brick/osbrick"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

func (a *agent) ConnectBlockDevice(ctx context.Context, req *pb.ConnectBlockDeviceRequest) (*pb.ConnectBlockDeviceResponse, error) {
	deviceName, err := osbrick.ConnectMultipathVolume(ctx, req.PortalAddresses, req.HostLunId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect block device: %+v", err)
	}

	return &pb.ConnectBlockDeviceResponse{
		DeviceName: deviceName,
	}, nil
}
