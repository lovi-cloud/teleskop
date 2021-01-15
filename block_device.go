package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lovi-cloud/go-os-brick/osbrick"
	pb "github.com/lovi-cloud/teleskop/protoc/agent"
)

func (a *agent) ConnectBlockDevice(ctx context.Context, req *pb.ConnectBlockDeviceRequest) (*pb.ConnectBlockDeviceResponse, error) {
	var deviceName string
	var err error

	switch len(req.PortalAddresses) {
	case 1:
		deviceName, err = osbrick.ConnectSinglePathVolume(ctx, req.PortalAddresses[0], int(req.HostLunId))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect block device: %+v", err)
		}
	default:
		deviceName, err = osbrick.ConnectMultipathVolume(ctx, req.PortalAddresses, int(req.HostLunId))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect block device: %+v", err)
		}
	}

	return &pb.ConnectBlockDeviceResponse{
		DeviceName: deviceName,
	}, nil
}

func (a *agent) DisconnectBlockDevice(ctx context.Context, req *pb.DisconnectBlockDeviceRequest) (*pb.DisconnectBlockDeviceResponse, error) {
	switch len(req.PortalAddresses) {
	case 1:
		if err := osbrick.DisconnectSinglePathVolume(ctx, req.PortalAddresses[0], int(req.HostLunId)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to disconnect block device: %+v", err)
		}
	default:
		if err := osbrick.DisconnectVolume(ctx, req.PortalAddresses, int(req.HostLunId)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to disconnect block device: %+v", err)
		}
	}

	return &pb.DisconnectBlockDeviceResponse{}, nil
}
