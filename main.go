//go:generate protoc -I ./protoc/agent --go_out=plugins=grpc:./protoc/agent ./protoc/agent/agent.proto

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"google.golang.org/grpc"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

const (
	listenAddress = ":5000"
)

type agent struct {
	pb.UnimplementedAgentServer

	libvirtClient *libvirt.Libvirt
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
	conn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:16509")
	if err != nil {
		return fmt.Errorf("failed to dial to libvirtd: %w", err)
	}

	libvirtClient := libvirt.New(conn)
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

	server := grpc.NewServer()
	pb.RegisterAgentServer(server, &agent{
		libvirtClient: libvirtClient,
	})

	fmt.Printf("listening on address %s\n", listenAddress)

	return server.Serve(lis)
}
