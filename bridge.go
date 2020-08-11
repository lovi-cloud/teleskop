package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
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

	_, err = a.libvirtClient.NetworkLookupByName(req.Name)
	if err == nil {
		// TODO: already exists
		return &pb.AddBridgeResponse{}, nil
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
		// TODO: not found
		return &pb.DeleteBridgeResponse{}, nil
	}

	if err := a.libvirtClient.NetworkDestroy(network); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to stop network: %+v", err)
	}

	if err := a.libvirtClient.NetworkUndefine(network); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to undefine network: %+v", err)
	}

	if err := deleteTeleskopInterfaceIfExists(ctx, req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete teleskop interface: %+v", err)
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

	if link.Attrs().MasterIndex != 0 {
		// TODO: already added
		return &pb.AddInterfaceToBridgeResponse{}, nil
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
	veth, vethPeer, err := createVethPeerIfNotExists(ctx, name)
	if err != nil {
		return err
	}

	bridge, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to find bridge name=%s: %w", name, err)
	}
	err = netlink.LinkSetMaster(veth, bridge)
	if err != nil {
		return fmt.Errorf("failed to set link master link=%s bridge=%s: %w", veth.Attrs().Name, bridge.Attrs().Name, err)
	}

	mask, _ := ipnet.Mask.Size()
	addr, err := netlink.ParseAddr(fmt.Sprintf("%s/%d", ip.String(), mask))
	if err != nil {
		return fmt.Errorf("failed to parse addr=\"%s/%d\": %w", ip.String(), mask, err)
	}
	err = netlink.AddrAdd(vethPeer, addr)
	if err != nil {
		return fmt.Errorf("failed to addr add link=%s, addr=%s: %w", vethPeer.Attrs().Name, addr, err)
	}

	err = netlink.LinkSetUp(veth)
	if err != nil {
		return fmt.Errorf("failed to set up link=%s: %w", veth.Attrs().Name, err)
	}
	err = netlink.LinkSetUp(vethPeer)
	if err != nil {
		return fmt.Errorf("failed to set up link=%s: %w", vethPeer.Attrs().Name, err)
	}

	return nil
}

func deleteTeleskopInterfaceIfExists(ctx context.Context, name string) error {
	veth, vethPeer, err := createVethPeerIfNotExists(ctx, name)
	if err != nil {
		return nil
	}

	err = netlink.LinkSetDown(veth)
	if err != nil {
		return fmt.Errorf("failed to set down link=%s: %w", veth.Attrs().Name, err)
	}
	err = netlink.LinkSetDown(vethPeer)
	if err != nil {
		return fmt.Errorf("failed to set down link=%s: %w", vethPeer.Attrs().Name, err)
	}

	addrs, err := netlink.AddrList(vethPeer, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to get addr list link=%s: %w", vethPeer.Attrs().Name, err)
	}
	for _, addr := range addrs {
		err = netlink.AddrDel(vethPeer, &addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete addr link=%s, addr=%s: %+v\n", vethPeer.Attrs().Name, addr, err)
		}
	}

	err = netlink.LinkSetNoMaster(veth)
	if err != nil {
		return fmt.Errorf("failed to set link no master link=%s: %w", veth.Attrs().Name, err)
	}

	err = netlink.LinkDel(veth)
	if err != nil {
		return fmt.Errorf("failed to delete link link=%s: %w", veth.Attrs().Name, err)
	}

	return nil
}

func createVethPeerIfNotExists(ctx context.Context, name string) (netlink.Link, netlink.Link, error) {
	var err error
	var veth, vethPeer netlink.Link

	vethName := fmt.Sprintf("%s-dhcp", name)
	vethPeerName := fmt.Sprintf("dhcp-%s", name)

	veth, err = netlink.LinkByName(vethName)
	if err != nil {
		veth = &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name: vethName,
			},
			PeerName: vethPeerName,
		}
		err2 := netlink.LinkAdd(veth)
		if err2 != nil {
			return nil, nil, fmt.Errorf("failed to add new link name=%s: %w", vethName, err)
		}
	}
	v, ok := veth.(*netlink.Veth)
	if !ok {
		return nil, nil, fmt.Errorf("invalid link name=%s", veth.Attrs().Name)
	}
	peerIndex, err := netlink.VethPeerIndex(v)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find veth peer index name=%s: %w", veth.Attrs().Name, err)
	}
	vethPeer, err = netlink.LinkByIndex(peerIndex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find veth peer link by index=%d: %w", peerIndex, err)
	}

	return veth, vethPeer, nil
}
