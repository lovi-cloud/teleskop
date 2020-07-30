package dhcp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	pb "github.com/whywaita/satelit/api/satelit_datastore"
	"go.universe.tf/netboot/dhcp4"
)

// Server is DHCP server
type Server struct {
	mutex  *sync.Mutex
	client pb.SatelitDatastoreClient
}

// NewServer is return new DHCP server
func NewServer(client pb.SatelitDatastoreClient) *Server {
	return &Server{
		mutex:  &sync.Mutex{},
		client: client,
	}
}

// ListenAndServe listens on the UDP network and serve DHCP server
func (s *Server) ListenAndServe() error {
	conn, err := dhcp4.NewConn("0.0.0.0:67")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer conn.Close()

	for {
		req, intf, err := conn.RecvDHCP()
		if err != nil {
			return fmt.Errorf("failed to receive DHCP request: %w", err)
		}
		if !strings.HasPrefix(intf.Name, "dhcp") {
			continue
		}

		lease, err := s.client.GetDHCPLease(context.Background(), &pb.GetDHCPLeaseRequest{
			MacAddress: req.HardwareAddr.String(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", err)
			continue
		}

		resp, err := makeResponse(*intf, *req, lease.Lease)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "%+v\n", resp)

		err = conn.SendDHCP(resp, intf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", err)
			continue
		}
	}
	// return nil
}

func makeResponse(intf net.Interface, req dhcp4.Packet, lease *pb.DHCPLease) (*dhcp4.Packet, error) {
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface address: %w", err)
	}
	var serverAddr *net.IP
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil || ip.To4() == nil {
			continue
		}
		serverAddr = &ip
	}
	if serverAddr == nil {
		return nil, fmt.Errorf("failed to parse interface address: %w", err)
	}

	yourAddr := net.ParseIP(lease.Ip)
	if yourAddr == nil {
		return nil, fmt.Errorf("failed to parse IP: %w", err)
	}

	_, netmask, err := net.ParseCIDR(lease.Network)
	if err != nil {
		return nil, fmt.Errorf("failed to parse network: %w", err)
	}

	resp := &dhcp4.Packet{
		TransactionID:  req.TransactionID,
		Broadcast:      req.Broadcast,
		HardwareAddr:   req.HardwareAddr,
		YourAddr:       yourAddr,
		ServerAddr:     *serverAddr,
		RelayAddr:      req.RelayAddr,
		BootServerName: serverAddr.String(),
	}
	options := make(dhcp4.Options)
	options[dhcp4.OptSubnetMask] = netmask.Mask
	options[dhcp4.OptServerIdentifier] = *serverAddr

	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, 4294967295)
	options[dhcp4.OptLeaseTime] = buff

	if lease.DnsServer != "" {
		addr := net.ParseIP(lease.DnsServer)
		if addr == nil {
			return nil, fmt.Errorf("failed to parse DNS: %s", lease.DnsServer)
		}
		options[dhcp4.OptDNSServers] = addr.To4()
	}
	options[121] = []byte{}
	if lease.MetadataServer != "" {
		addr := net.ParseIP(lease.MetadataServer)
		if addr == nil {
			return nil, fmt.Errorf("failed to parse metadata server: %s", lease.MetadataServer)
		}
		md := addr.To4()
		options[121] = append(options[121], []byte{32, 169, 254, 169, 254, md[0], md[1], md[2], md[3]}...)
	}
	if lease.Gateway != "" {
		addr := net.ParseIP(lease.Gateway)
		if addr == nil {
			return nil, fmt.Errorf("failed to parse gateway: %s", lease.Gateway)
		}
		gw := addr.To4()
		options[dhcp4.OptRouters] = gw
		options[121] = append(options[121], []byte{0, gw[0], gw[1], gw[2], gw[3]}...)
	}
	resp.Options = options

	switch req.Type {
	case dhcp4.MsgDiscover:
		resp.Type = dhcp4.MsgOffer
	case dhcp4.MsgRequest:
		resp.Type = dhcp4.MsgAck
	}

	return resp, nil
}
