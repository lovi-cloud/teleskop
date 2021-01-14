package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	pb "github.com/lovi-cloud/satelit/api/satelit_datastore"
)

const nodePattern = "/sys/devices/system/node/node*"

// Error variables
var (
	ErrInvalidNUMATopology = fmt.Errorf("invalid NUMA topology")
	ErrInvalidCPUList      = fmt.Errorf("invalid cpu list")
)

// GetLocalNUMANodes retrieve info of local NUMA nodes and CPU cores.
func GetLocalNUMANodes() ([]*pb.NumaNode, error) {
	nodes, err := filepath.Glob(nodePattern)
	if err != nil {
		return nil, err
	}

	n := make([]*pb.NumaNode, len(nodes))
	for i, node := range nodes {
		tmp, err := ioutil.ReadFile(filepath.Join(node, "cpulist"))
		if err != nil {
			return nil, err
		}
		cpu, err := ParseNodeList(string(tmp))
		if err != nil {
			return nil, err
		}
		n[i] = cpu
	}

	return n, nil
}

// ParseNodeList parsed content of cpulist
// ex:) 0-9,40-49
func ParseNodeList(cpulist string) (*pb.NumaNode, error) {
	coreIDs := strings.Split(cpulist, ",")
	switch len(coreIDs) {
	case 1:
		return parseOneNodeLine(coreIDs)
	case 2:
		return parseTwoNodeList(coreIDs)
		// Maybe can input more double NUMA Node (for example: 4,8...), but not implement yet. Because not used by me.
		// please contribute if you need.
	default:
		return nil, ErrInvalidNUMATopology
	}
}

func parseOneNodeLine(coreIDs []string) (*pb.NumaNode, error) {
	if len(coreIDs) != 1 {
		// don't allow not one NUMA node
		return nil, ErrInvalidNUMATopology
	}

	list, err := extractCoreID(coreIDs[0])
	if err != nil {
		return nil, err
	}

	node := pb.NumaNode{
		Pairs:           make([]*pb.CorePair, len(list)),
		PhysicalCoreMin: math.MaxUint32,
		PhysicalCoreMax: 0,
		LogicalCoreMin:  nil,
		LogicalCoreMax:  nil,
	}
	for _, core := range list {
		if core < node.PhysicalCoreMin {
			node.PhysicalCoreMin = core
		}
		if core > node.PhysicalCoreMax {
			node.PhysicalCoreMax = core
		}
	}

	for i := 0; i < len(list); i++ {
		node.Pairs[i] = &pb.CorePair{
			PhysicalCore: list[i],
			LogicalCore:  nil,
		}
	}

	return &node, nil
}

func parseTwoNodeList(coreIDs []string) (*pb.NumaNode, error) {
	if len(coreIDs) != 2 {
		// don't allow not two NUMA node
		return nil, ErrInvalidNUMATopology
	}

	list1, err := extractCoreID(coreIDs[0])
	if err != nil {
		return nil, err
	}
	list2, err := extractCoreID(coreIDs[1])
	if err != nil {
		return nil, err
	}
	if len(list1) != len(list2) {
		return nil, ErrInvalidCPUList
	}

	return NewNode(list1, list2)
}

// NewNode create NUMA node.
func NewNode(physicalCoreList, logicalCoreList []uint32) (*pb.NumaNode, error) {
	if len(physicalCoreList) != len(logicalCoreList) {
		return nil, fmt.Errorf("invalid core list")
	}

	maxUint32 := uint32(math.MaxUint32)
	zero := uint32(0)

	node := pb.NumaNode{
		Pairs:           make([]*pb.CorePair, len(physicalCoreList)),
		PhysicalCoreMin: math.MaxUint32,
		PhysicalCoreMax: 0,
		LogicalCoreMin:  &maxUint32,
		LogicalCoreMax:  &zero,
	}
	for _, pc := range physicalCoreList {
		if pc < node.PhysicalCoreMin {
			node.PhysicalCoreMin = pc
		}
		if pc > node.PhysicalCoreMax {
			node.PhysicalCoreMax = pc
		}
	}
	for _, lc := range logicalCoreList {
		if lc < *node.LogicalCoreMin {
			node.LogicalCoreMin = &lc
		}
		if lc > *node.LogicalCoreMax {
			node.LogicalCoreMax = &lc
		}
	}
	for i := 0; i < len(physicalCoreList); i++ {
		node.Pairs[i] = &pb.CorePair{
			PhysicalCore: physicalCoreList[i],
			LogicalCore:  &(logicalCoreList[i]),
		}
	}

	return &node, nil
}

func extractCoreID(s string) (coreIDs []uint32, err error) {
	s = strings.TrimSpace(s)
	num := strings.Split(s, "-")
	if len(num) < 2 {
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, err
		}
		return []uint32{uint32(n)}, nil
	}
	start, err := strconv.ParseInt(num[0], 10, 32)
	if err != nil {
		return nil, err
	}
	end, err := strconv.ParseInt(num[1], 10, 32)
	if err != nil {
		return nil, err
	}
	for i := start; i <= end; i++ {
		coreIDs = append(coreIDs, uint32(i))
	}

	return coreIDs, nil
}
