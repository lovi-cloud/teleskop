package main

import (
	"context"
	"fmt"
	"net"

	"github.com/coreos/go-iptables/iptables"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

const (
	maxChainNameLength = 29

	tableFilter = "filter"

	chainINPUT                = "INPUT"
	chainFORWARD              = "FORWARD"
	chainCallistoSGFallback   = "callisto-sg-fallback"
	chainCallistoSG           = "callisto-sg-chain"
	chainCallistoINPUT        = "callisto-INPUT"
	chainCallistoFORWARD      = "callisto-FORWARD"
	chainCallistoINPUTPrefix  = "callisto-i"
	chainCallistoOUTPUPrefix  = "callisto-o"
	chainCallistoSOURCEPrefix = "callisto-s"

	actionACCEPT = "ACCEPT"
	actionDROP   = "DROP"
	actionRETURN = "RETURN"
)

var (
	setupFunctions = []setupFunction{
		setupSGFallbackChain,
		setupSGChain,
		setupINPUT,
		setupFORWARD,
	}
	addFunctions = []addFunction{
		addSOURCESGRules,
		addOUTPUTSGRules,
		addINPUTSGRules,
		addSGRules,
		addINPUTRules,
		addFORWARDRules,
	}

	ruleSGFallback = []string{
		"-m", "comment", "--comment", "Default drop rule for unmatched traffic.", "-j", actionDROP,
	}
	ruleINPUT = []string{
		"-j", chainCallistoINPUT,
	}
	ruleFORWARD = []string{
		"-j", chainCallistoFORWARD,
	}
)

type setupFunction func(ctx context.Context, client *iptables.IPTables) error
type addFunction func(ctx context.Context, client *iptables.IPTables, intf link) error

type link struct {
	Name       string
	IPAddress  net.IP
	MACAddress net.HardwareAddr
}

func (a *agent) GetIPTables(ctx context.Context, req *pb.GetIPTablesRequest) (*pb.GetIPTablesResponse, error) {
	return &pb.GetIPTablesResponse{}, nil
}

func (a *agent) SetupDefaultSecurityGroup(ctx context.Context, req *pb.SetupDefaultSecurityGroupRequest) (*pb.SetupDefaultSecurityGroupResponse, error) {
	client, err := iptables.New()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create iptables client: %+v", err)
	}

	if err := setupDefaultSecurityGroup(ctx, client); err != nil {
		return nil, err
	}

	return &pb.SetupDefaultSecurityGroupResponse{}, nil
}

func (a *agent) AddSecurityGroup(ctx context.Context, req *pb.AddSecurityGroupRequest) (*pb.AddSecurityGroupResponse, error) {
	client, err := iptables.New()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create iptables client: %+v", err)
	}

	ipAddr := net.ParseIP(req.IpAddress)
	if ipAddr == nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse request IP address: %s", "")
	}

	macAddr, err := net.ParseMAC(req.MacAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse request MAC address: %+v", err)
	}

	err = addSecurityGroup(ctx, client, link{
		Name:       req.Interface,
		IPAddress:  ipAddr,
		MACAddress: macAddr,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add security group: %+v", err)
	}

	return &pb.AddSecurityGroupResponse{}, nil
}

func setupDefaultSecurityGroup(ctx context.Context, client *iptables.IPTables) error {
	for _, fn := range setupFunctions {
		if err := fn(ctx, client); err != nil {
			return err
		}
	}
	return nil
}

func addSecurityGroup(ctx context.Context, client *iptables.IPTables, intf link) error {
	for _, fn := range addFunctions {
		if err := fn(ctx, client, intf); err != nil {
			return err
		}
	}
	return nil
}

func setupChain(ctx context.Context, client *iptables.IPTables, name string) error {
	if err := client.NewChain(tableFilter, name); err != nil {
		return status.Errorf(codes.Internal, "failed to create new chain: %+v", err)
	}
	return nil
}

func setupSGFallbackChain(ctx context.Context, client *iptables.IPTables) error {
	if err := setupChain(ctx, client, chainCallistoSGFallback); err != nil {
		return err
	}

	if err := client.AppendUnique(tableFilter, chainCallistoSGFallback, ruleSGFallback...); err != nil {
		return status.Errorf(codes.Internal, "failed to append new rule: %+v", err)
	}

	return nil
}

func setupSGChain(ctx context.Context, client *iptables.IPTables) error {
	if err := setupChain(ctx, client, chainCallistoSG); err != nil {
		return err
	}

	if err := client.AppendUnique(tableFilter, chainCallistoSG,
		"-j", actionACCEPT,
	); err != nil {
		return status.Errorf(codes.Internal, "failed to append new rule: %+v", err)
	}

	return nil
}

func setupINPUT(ctx context.Context, client *iptables.IPTables) error {
	if err := setupChain(ctx, client, chainCallistoINPUT); err != nil {
		return err
	}

	if err := client.AppendUnique(tableFilter, chainINPUT, ruleINPUT...); err != nil {
		return status.Errorf(codes.Internal, "failed to append new rule: %+v", err)
	}

	return nil
}

func setupFORWARD(ctx context.Context, client *iptables.IPTables) error {
	if err := setupChain(ctx, client, chainCallistoFORWARD); err != nil {
		return err
	}

	if err := client.AppendUnique(tableFilter, chainFORWARD, ruleFORWARD...); err != nil {
		return status.Errorf(codes.Internal, "failed to append new rule: %+v", err)
	}

	return nil
}

func addSOURCESGRules(ctx context.Context, client *iptables.IPTables, intf link) error {
	var err error

	chain := getSOURCEChainName(intf)
	if err := setupChain(ctx, client, chain); err != nil {
		return err
	}

	rules := [][]string{
		{"-s", fmt.Sprintf("%s/32", intf.IPAddress), "-m", "mac", "--mac-source", intf.MACAddress.String(), "-m", "comment", "--comment", "Allow traffic from defined IP/MAC pairs.", "-j", actionRETURN},
		{"-m", "comment", "--comment", "Drop traffic without an IP/MAC allow rule.", "-j", actionDROP},
	}
	for i, rule := range rules {
		err = client.Insert(tableFilter, chain, i+1, rule...)
		if err != nil {
			client.ClearChain(tableFilter, chain)
			client.DeleteChain(tableFilter, chain)
			return status.Errorf(codes.Internal, "failed to append new %s rule: %+v", chain, err)
		}
	}

	return nil
}

func addINPUTSGRules(ctx context.Context, client *iptables.IPTables, intf link) error {
	var err error

	chain := getINPUTChainName(intf)
	if err := setupChain(ctx, client, chain); err != nil {
		return err
	}

	rules := [][]string{
		{"-m", "state", "--state", "RELATED,ESTABLISHED", "-m", "comment", "--comment", "Direct packets associated with a known session to the RETURN chain.", "-j", actionRETURN},
		{"-p", "udp", "-m", "udp", "--sport", "67", "--dport", "68", "-j", actionRETURN},
		{"-p", "tcp", "-m", "tcp", "-m", "multiport", "--dports", "1:65535", "-j", actionRETURN},
		{"-p", "udp", "-m", "udp", "-m", "multiport", "--dports", "1:65535", "-j", actionRETURN},
		{"-p", "icmp", "-j", actionRETURN},
		{"-m", "comment", "--comment", "Send unmatched traffic to the fallback chain.", "-j", chainCallistoSGFallback},
	}
	for i, rule := range rules {
		err = client.Insert(tableFilter, chain, i+1, rule...)
		if err != nil {
			client.ClearChain(tableFilter, chain)
			client.DeleteChain(tableFilter, chain)
			return status.Errorf(codes.Internal, "failed to append new %s rule: %+v", chain, err)
		}
	}

	return nil
}

func addOUTPUTSGRules(ctx context.Context, client *iptables.IPTables, intf link) error {
	var err error

	chain := getOUTPUTChainName(intf)
	if err := setupChain(ctx, client, chain); err != nil {
		return err
	}

	rules := [][]string{
		{"-p", "udp", "-m", "udp", "--sport", "68", "--dport", "67", "-m", "comment", "--comment", "Allow DHCP client traffic.", "-j", actionRETURN},
		{"-j", getSOURCEChainName(intf)},
		{"-p", "udp", "-m", "udp", "--sport", "67", "--dport", "68", "-m", "comment", "--comment", "Prevent DHCP Spoofing by VM.", "-j", actionDROP},
		{"-m", "state", "--state", "RELATED,ESTABLISHED", "-m", "comment", "--comment", "Direct packets associated with a known session to the RETURN chain.", "-j", actionRETURN},
		{"-p", "tcp", "-m", "tcp", "-m", "multiport", "--dports", "1:65535", "-j", actionRETURN},
		{"-p", "udp", "-m", "udp", "-m", "multiport", "--dports", "1:65535", "-j", actionRETURN},
		{"-p", "icmp", "-j", actionRETURN},
		{"-j", actionRETURN},
		{"-m", "comment", "--comment", "Send unmatched traffic to the fallback chain.", "-j", chainCallistoSGFallback},
	}
	for i, rule := range rules {
		err = client.Insert(tableFilter, chain, i+1, rule...)
		if err != nil {
			client.ClearChain(tableFilter, chain)
			client.DeleteChain(tableFilter, chain)
			return status.Errorf(codes.Internal, "failed to append new %s rule: %+v", chain, err)
		}
	}

	return nil
}

func addSGRules(ctx context.Context, client *iptables.IPTables, intf link) error {
	var err error

	chain := chainCallistoSG
	rules := [][]string{
		{"-m", "physdev", "--physdev-out", intf.Name, "--physdev-is-bridged", "-m", "comment", "--comment", "Jump to the VM specific chain.", "-j", getINPUTChainName(intf)},
		{"-m", "physdev", "--physdev-in", intf.Name, "--physdev-is-bridged", "-m", "comment", "--comment", "Jump to the VM specific chain.", "-j", getOUTPUTChainName(intf)},
	}
	for i, rule := range rules {
		err = client.Insert(tableFilter, chain, i+1, rule...)
		if err != nil {
			client.ClearChain(tableFilter, chain)
			client.DeleteChain(tableFilter, chain)
			return status.Errorf(codes.Internal, "failed to append new %s rule: %+v", chain, err)
		}
	}

	return nil
}

func addINPUTRules(ctx context.Context, client *iptables.IPTables, intf link) error {
	var err error

	chain := chainCallistoFORWARD
	rules := [][]string{
		{"-m", "physdev", "--physdev-in", intf.Name, "--physdev-is-bridged", "-m", "comment", "--comment", "Direct incoming traffic from VM to the security group chain.", "-j", getOUTPUTChainName(intf)},
	}
	for i, rule := range rules {
		err = client.Insert(tableFilter, chain, i+1, rule...)
		if err != nil {
			client.ClearChain(tableFilter, chain)
			client.DeleteChain(tableFilter, chain)
			return status.Errorf(codes.Internal, "failed to append new %s rule: %+v", chain, err)
		}
	}

	return nil
}

func addFORWARDRules(ctx context.Context, client *iptables.IPTables, intf link) error {
	var err error

	chain := chainCallistoFORWARD
	rules := [][]string{
		{"-m", "physdev", "--physdev-out", intf.Name, "--physdev-is-bridged", "-m", "comment", "--comment", "Direct traffic from the VM interface to the security group chain.", "-j", chainCallistoSG},
		{"-m", "physdev", "--physdev-in", intf.Name, "--physdev-is-bridged", "-m", "comment", "--comment", "Direct traffic from the VM interface to the security group chain.", "-j", chainCallistoSG},
	}
	for i, rule := range rules {
		err = client.Insert(tableFilter, chain, i+1, rule...)
		if err != nil {
			client.ClearChain(tableFilter, chain)
			client.DeleteChain(tableFilter, chain)
			return status.Errorf(codes.Internal, "failed to append new %s rule: %+v", chain, err)
		}
	}

	return nil
}

func getINPUTChainName(intf link) string {
	return getValidChainName(chainCallistoINPUTPrefix, intf.Name)
}

func getOUTPUTChainName(intf link) string {
	return getValidChainName(chainCallistoOUTPUPrefix, intf.Name)
}

func getSOURCEChainName(intf link) string {
	return getValidChainName(chainCallistoSOURCEPrefix, intf.Name)
}

func getValidChainName(prefix, val string) string {
	tmp := fmt.Sprintf("%s%s", prefix, val)
	if len(tmp) > maxChainNameLength {
		return tmp[:maxChainNameLength]
	}
	return tmp
}
