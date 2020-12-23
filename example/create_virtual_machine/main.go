package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lovi-cloud/teleskop/example"

	pb "github.com/lovi-cloud/teleskop/protoc/agent"
)

const (
	instanceName         = "yjuba-instance001"
	instanceVCPUs        = 1
	instanceMemoryKib    = 10004480
	instanceBootDevice   = "/dev/loop2"
	instanceInterface    = "tap001"
	instanceInterfaceMAC = "ca:61:85:97:93:e2"

	bridgeName     = "br1000"
	bandwidthLimit = 128000
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

	resp, err := client.AddVirtualMachine(ctx, &pb.AddVirtualMachineRequest{
		Name:       instanceName,
		Vcpus:      instanceVCPUs,
		MemoryKib:  instanceMemoryKib,
		BootDevice: instanceBootDevice,
	})
	if err != nil {
		return fmt.Errorf("failed to add virtual machine: %w", err)
	}

	_, err = client.AttachInterface(ctx, &pb.AttachInterfaceRequest{
		Uuid:            resp.Uuid,
		Bridge:          bridgeName,
		InboundAverage:  bandwidthLimit,
		OutboundAverage: bandwidthLimit,
		Name:            instanceInterface,
		MacAddress:      instanceInterfaceMAC,
	})
	if err != nil {
		return fmt.Errorf("failed to attach interfce: %w", err)
	}

	// TODO: ipman
	_, err = client.AddSecurityGroup(ctx, &pb.AddSecurityGroupRequest{
		Interface:  instanceInterface,
		IpAddress:  "10.192.0.100",
		MacAddress: instanceInterfaceMAC,
	})
	if err != nil {
		return fmt.Errorf("failed to add security group: %w", err)
	}

	_, err = client.StartVirtualMachine(ctx, &pb.StartVirtualMachineRequest{
		Uuid: resp.Uuid,
	})
	if err != nil {
		return fmt.Errorf("failed to start virtual machine: %w", err)
	}

	return nil
}
