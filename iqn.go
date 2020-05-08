package main

import (
	"context"
	"io/ioutil"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

const iqnFilePath = "/etc/iscsi/initiatorname.iscsi"

func (a *agent) GetISCSIQualifiedName(ctx context.Context, req *pb.GetISCSIQualifiedNameRequest) (*pb.GetISCSIQualifiedNameResponse, error) {
	b, err := ioutil.ReadFile(iqnFilePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read iqn file: %+v", err)
	}

	tmp := strings.TrimSpace(string(b))
	words := strings.Split(tmp, "=")
	if len(words) < 2 {
		return nil, status.Errorf(codes.Internal, "failed to parse iqn file: %s", tmp)
	}

	return &pb.GetISCSIQualifiedNameResponse{
		Iqn: words[1],
	}, nil
}
