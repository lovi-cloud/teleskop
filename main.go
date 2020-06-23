//go:generate protoc -I ./protoc/agent --go_out=plugins=grpc:./protoc/agent ./protoc/agent/agent.proto

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	dspb "github.com/whywaita/satelit/api/satelit_datastore"
	"github.com/whywaita/teleskop/dhcp"
	pb "github.com/whywaita/teleskop/protoc/agent"
)

const (
	listenAddress = ":5000"
)

type agent struct {
	pb.UnimplementedAgentServer

	libvirtClient *libvirt.Libvirt
	dhcpServer    *dhcp.Server
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var dialer net.Dialer
	libvirtConn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:16509")
	if err != nil {
		return fmt.Errorf("failed to dial to libvirtd: %w", err)
	}

	libvirtClient := libvirt.New(libvirtConn)
	if err := libvirtClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to libvirtd: %w", err)
	}

	libvirtVersion, err := libvirtClient.ConnectGetLibVersion()
	if err != nil {
		return fmt.Errorf("failed to get libvirtd versoin: %w", err)
	}
	fmt.Printf("connect to libvirtd version = %d\n", libvirtVersion)

	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen address: %w", err)
	}

	grpcConn, err := grpc.DialContext(ctx,
		"10.194.228.99:9263",
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial to satelit datastore api: %w", err)
	}
	datastoreClient := dspb.NewSatelitDatastoreClient(grpcConn)

	server := grpc.NewServer()
	dhcpServer := dhcp.NewServer(datastoreClient)
	pb.RegisterAgentServer(server, &agent{
		libvirtClient: libvirtClient,
		dhcpServer:    dhcpServer,
	})

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		fmt.Printf("listening on address %s\n", listenAddress)
		return server.Serve(lis)
	})
	eg.Go(func() error {
		fmt.Printf("listening on address %s\n", "0.0.0.0:67")
		return dhcpServer.ListenAndServe()
	})

	return eg.Wait()
}
