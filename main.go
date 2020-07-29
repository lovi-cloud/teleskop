//go:generate protoc -I ./protoc/agent --go_out=plugins=grpc:./protoc/agent ./protoc/agent/agent.proto

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	libvirt "github.com/digitalocean/go-libvirt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
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

func initLogger() (*zap.Logger, error) {
	return zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.InfoLevel),
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "Time",
			LevelKey:       "Level",
			NameKey:        "Name",
			CallerKey:      "Caller",
			MessageKey:     "Msg",
			StacktraceKey:  "St",
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	logger, err := initLogger()
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

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
		"10.197.32.54:9263",
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial to satelit datastore api: %w", err)
	}
	datastoreClient := dspb.NewSatelitDatastoreClient(grpcConn)

	opts := []grpc_zap.Option{
		grpc_zap.WithMessageProducer(grpc_zap.DefaultMessageProducer),
	}
	grpc_zap.ReplaceGrpcLoggerV2(logger)
	grpcServer := grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_zap.PayloadUnaryServerInterceptor(logger, func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
				return true
			}),
			grpc_zap.UnaryServerInterceptor(logger, opts...),
		),
		grpc_middleware.WithStreamServerChain(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_zap.PayloadStreamServerInterceptor(logger, func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
				return true
			}),
			grpc_zap.StreamServerInterceptor(logger, opts...),
		),
	)
	dhcpServer := dhcp.NewServer(datastoreClient)
	pb.RegisterAgentServer(grpcServer, &agent{
		libvirtClient: libvirtClient,
		dhcpServer:    dhcpServer,
	})

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		fmt.Printf("listening on address %s\n", listenAddress)
		return grpcServer.Serve(lis)
	})
	eg.Go(func() error {
		fmt.Printf("listening on address %s\n", "0.0.0.0:67")
		return dhcpServer.ListenAndServe()
	})

	return eg.Wait()
}
