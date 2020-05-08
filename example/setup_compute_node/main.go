package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/whywaita/teleskop/example"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

const (
	vlanID          = 1000
	parentInterface = "bond0"
	bridgeName      = "br1000"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr = flag.String("addr", "127.0.0.1:5000", "teleskop agent address.")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := example.SetupClient(ctx, *addr)
	if err != nil {
		return err
	}

	_, err = client.AddVLANInterface(ctx, &pb.AddVLANInterfaceRequest{
		VlanId:          vlanID,
		ParentInterface: parentInterface,
	})
	if err != nil {
		return fmt.Errorf("failed to add VLAN interface: %w", err)
	}

	_, err = client.AddBridge(ctx, &pb.AddBridgeRequest{
		Name: bridgeName,
	})
	if err != nil {
		return fmt.Errorf("failed to add bridge: %w", err)
	}

	_, err = client.AddInterfaceToBridge(ctx, &pb.AddInterfaceToBridgeRequest{
		Bridge:    bridgeName,
		Interface: fmt.Sprintf("%s.%d", parentInterface, vlanID),
	})
	if err != nil {
		return fmt.Errorf("failed to add interface to bridge: %w", err)
	}

	_, err = client.SetupDefaultSecurityGroup(ctx, &pb.SetupDefaultSecurityGroupRequest{})
	if err != nil {
		return fmt.Errorf("failed to setup default security group: %w", err)
	}

	return nil
}
