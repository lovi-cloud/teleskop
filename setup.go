package main

import (
	"context"
	"fmt"

	dspb "github.com/whywaita/satelit/api/satelit_datastore"
	pb "github.com/whywaita/teleskop/protoc/agent"
)

func (a *agent) setup(ctx context.Context, hostname, endpoint string) error {
	resp, err := a.datastoreClient.ListBridge(ctx, &dspb.ListBridgeRequest{})
	if err != nil {
		return err
	}

	for _, bridge := range resp.Bridges {
		_, err = a.AddBridge(ctx, &pb.AddBridgeRequest{
			Name:         bridge.Name,
			MetadataCidr: bridge.MetadataCidr,
			InternalOnly: bridge.InternalOnly,
		})
		if err != nil {
			return err
		}
		if bridge.InternalOnly {
			continue
		}
		_, err = a.AddVLANInterface(ctx, &pb.AddVLANInterfaceRequest{
			VlanId:          bridge.VlanId,
			ParentInterface: bridge.ParentInterface,
		})
		if err != nil {
			return err
		}
		_, err = a.AddInterfaceToBridge(ctx, &pb.AddInterfaceToBridgeRequest{
			Bridge:    bridge.Name,
			Interface: fmt.Sprintf("%s.%d", bridge.ParentInterface, bridge.VlanId),
		})
		if err != nil {
			return err
		}
	}

	_, err = a.datastoreClient.RegisterTeleskopAgent(ctx, &dspb.RegisterTeleskopAgentRequest{
		Hostname: hostname,
		Endpoint: endpoint,
	})
	if err != nil {
		return err
	}

	return nil
}