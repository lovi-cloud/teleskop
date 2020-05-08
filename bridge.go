package main

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/vishvananda/netlink"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

const networkTmplStr = `
<network>
  <name>{{.Name}}</name>
  <bridge name='{{.Name}}' stp='off' delay='0'/>
</network>
`

var networkTmpl *template.Template

func (a *agent) AddBridge(ctx context.Context, req *pb.AddBridgeRequest) (*pb.AddBridgeResponse, error) {
	if networkTmpl == nil {
		tmp, err := template.New("networkTmpl").Parse(networkTmplStr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse network template: %+v", err)
		}
		networkTmpl = tmp
	}

	var buff bytes.Buffer
	if err := networkTmpl.Execute(&buff, req); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute network template: %+v", err)
	}

	network, err := a.libvirtClient.NetworkDefineXML(buff.String())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to define network: %+v", err)
	}

	if err := a.libvirtClient.NetworkCreate(network); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start network: %+v", err)
	}

	fmt.Printf("starting network: %+v\n", network)

	return &pb.AddBridgeResponse{}, nil
}

func (a *agent) DeleteBridge(ctx context.Context, req *pb.DeleteBridgeRequest) (*pb.DeleteBridgeResponse, error) {
	network, err := a.libvirtClient.NetworkLookupByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to find network: %+v", err)
	}

	if err := a.libvirtClient.NetworkDestroy(network); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to stop network: %+v", err)
	}

	if err := a.libvirtClient.NetworkUndefine(network); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to undefine network: %+v", err)
	}

	fmt.Printf("stopped network: %+v\n", network)

	return &pb.DeleteBridgeResponse{}, nil
}

func (a *agent) AddInterfaceToBridge(ctx context.Context, req *pb.AddInterfaceToBridgeRequest) (*pb.AddInterfaceToBridgeResponse, error) {
	bridge, err := netlink.LinkByName(req.Bridge)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to find bridge: %+v", err)
	}

	link, err := netlink.LinkByName(req.Interface)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to find interface: %+v", err)
	}

	if err := netlink.LinkSetMaster(link, bridge); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add interface to bridge: %+v", err)
	}

	return &pb.AddInterfaceToBridgeResponse{}, nil
}

func (a *agent) DeleteInterfaceFromBridge(ctx context.Context, req *pb.DeleteInterfaceFromBridgeRequest) (*pb.DeleteInterfaceFromBridgeResponse, error) {
	link, err := netlink.LinkByName(req.Interface)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to find interface: %+v", err)
	}

	if err := netlink.LinkSetNoMaster(link); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete interface from bridge: %+v", err)
	}

	return &pb.DeleteInterfaceFromBridgeResponse{}, nil
}
