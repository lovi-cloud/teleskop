package main

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

func (a *agent) AddVLANInterface(ctx context.Context, req *pb.AddVLANInterfaceRequest) (*pb.AddVLANInterfaceResponse, error) {
	parentLink, err := netlink.LinkByName(req.ParentInterface)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to find parent interface: %+v\n", err)
	}

	vlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        fmt.Sprintf("%s.%d", parentLink.Attrs().Name, req.VlanId),
			ParentIndex: parentLink.Attrs().Index,
		},
		VlanId:       int(req.VlanId),
		VlanProtocol: netlink.VLAN_PROTOCOL_8021Q,
	}

	if err := netlink.LinkAdd(vlan); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create vlan interface: %+v", err)
	}

	if err := netlink.LinkSetUp(vlan); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set up vlan interface: %+v", err)
	}

	return &pb.AddVLANInterfaceResponse{}, nil
}

func (a *agent) DeleteVLANInterface(ctx context.Context, req *pb.DeleteVLANInterfaceRequest) (*pb.DeleteVLANInterfaceResponse, error) {
	link, err := netlink.LinkByName(fmt.Sprintf("%s.%d", req.ParentInterface, req.VlanId))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to find vlan interface: %+v\n", err)
	}

	if err := netlink.LinkSetDown(link); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set down vlan interface: %+v", err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete vlan interface: %+v", err)
	}

	return &pb.DeleteVLANInterfaceResponse{}, nil
}
