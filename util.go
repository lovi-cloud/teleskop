package main

import (
	"encoding/hex"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libvirt "github.com/digitalocean/go-libvirt"
)

func (a *agent) domainLookupByUUID(uuidStr string) (*libvirt.Domain, error) {
	var uuid libvirt.UUID
	if _, err := hex.Decode(uuid[:], []byte(uuidStr)); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse uuid string: %+v", err)
	}

	domain, err := a.libvirtClient.DomainLookupByUUID(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lookup domain: %+v", err)
	}

	return &domain, nil
}
