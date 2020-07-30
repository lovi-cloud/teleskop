package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
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

	var (
		ip    net.IP
		ipnet *net.IPNet
		err   error
	)
	if !req.InternalOnly {
		ip, ipnet, err = net.ParseCIDR(req.MetadataCidr)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to parse request metadata cidr")
		}
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

	if !req.InternalOnly {
		err := addTeleskopInterface(ctx, req.Name, ip, ipnet)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to add dhcp interface: %+v", err)
		}
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

func addTeleskopInterface(ctx context.Context, name string, ip net.IP, ipnet *net.IPNet) error {
	var err error
	veth0 := fmt.Sprintf("%s-dhcp", name)
	veth1 := fmt.Sprintf("dhcp-%s", name)
	err = ipCmd(ctx, "link", "add", veth0, "type", "veth", "peer", "name", veth1)
	if err != nil {
		return err
	}
	err = brctlCmd(ctx, "addif", name, veth0)
	if err != nil {
		return err
	}
	err = ipCmd(ctx, "link", "set", "up", veth0)
	if err != nil {
		return err
	}
	err = ipCmd(ctx, "link", "set", "up", veth1)
	if err != nil {
		return err
	}
	mask, _ := ipnet.Mask.Size()
	addr := fmt.Sprintf("%s/%d", ip.String(), mask)
	err = ipCmd(ctx, "addr", "add", addr, "dev", veth1)
	if err != nil {
		return err
	}

	return nil
}

func ipCmd(ctx context.Context, arg ...string) error {
	return execCmd(ctx, "ip", arg...)
}

func brctlCmd(ctx context.Context, arg ...string) error {
	return execCmd(ctx, "brctl", arg...)
}

func execCmd(ctx context.Context, name string, arg ...string) error {
	out, err := exec.CommandContext(ctx, name, arg...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to exec \"%s %s\", msg=\"%s\": %w", name, strings.Join(arg, " "), string(out), err)
	}
	return nil
}
