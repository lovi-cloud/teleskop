package scheduler

import (
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const nodePattern = "/sys/devices/system/node/node*"

var (
	// ErrNoValidCoreFound is
	ErrNoValidCoreFound = fmt.Errorf("no valid core found")
	// ErrInvalidCorePair is
	ErrInvalidCorePair = fmt.Errorf("invalid core pair")
)

// GetLocalCPUList is
func GetLocalCPUList() ([]CPU, error) {
	nodes, err := filepath.Glob(nodePattern)
	if err != nil {
		return nil, err
	}

	CPUs := make([]CPU, len(nodes))
	for i, node := range nodes {
		tmp, err := ioutil.ReadFile(filepath.Join(node, "cpulist"))
		if err != nil {
			return nil, err
		}
		cpu, err := ParseCPUList(string(tmp))
		if err != nil {
			return nil, err
		}
		CPUs[i] = *cpu
	}

	return CPUs, nil
}

// ParseCPUList is
func ParseCPUList(cpulist string) (*CPU, error) {
	coreIDs := strings.Split(cpulist, ",")
	if len(coreIDs) != 2 {
		return nil, fmt.Errorf("invalid NUMA topology")
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
		return nil, fmt.Errorf("invalid cpu list")
	}

	return NewCPU(list1, list2)
}

// Scheduler is
type Scheduler struct {
	mutex *sync.Mutex
	cpus  []CPU
}

// NewScheduler is
func NewScheduler(cpuList []CPU) *Scheduler {
	return &Scheduler{
		mutex: &sync.Mutex{},
		cpus:  cpuList,
	}
}

// PopCorePair is
func (s *Scheduler) PopCorePair(num int) (pairs []CorePair, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for i := 0; i < len(s.cpus); i++ {
		if s.cpus[i].AvailableCorePair() < num {
			continue
		}
		pairs = make([]CorePair, num)
		for j := 0; j < num; j++ {
			pair, err := s.cpus[i].PopCorePair()
			if err != nil {
				return nil, err
			}
			pairs[j] = *pair
		}
		return pairs, nil
	}
	return nil, ErrNoValidCoreFound
}

// PushCorePair is
func (s *Scheduler) PushCorePair(pairs []CorePair) (err error) {
	for _, pair := range pairs {
		for i := 0; i < len(s.cpus); i++ {
			err = s.cpus[i].PushCorePair(pair)
			if err == ErrInvalidCorePair {
				continue
			} else if err != nil {
				return err
			}
			break
		}
	}
	return err
}

// CorePair is
type CorePair struct {
	PhysicalCore int
	LogicalCore  int
}

// CPU is
type CPU struct {
	corePairs       []CorePair
	physicalCoreMin int
	physicalCoreMax int
	logicalCoreMin  int
	logicalCoreMax  int
}

// NewCPU is
func NewCPU(physicalCoreList, logicalCoreList []int) (*CPU, error) {
	if len(physicalCoreList) != len(logicalCoreList) {
		return nil, fmt.Errorf("invalid core list")
	}

	cpu := CPU{
		corePairs:       make([]CorePair, len(physicalCoreList)),
		physicalCoreMin: math.MaxInt64,
		physicalCoreMax: math.MinInt64,
		logicalCoreMin:  math.MaxInt64,
		logicalCoreMax:  math.MinInt64,
	}
	for _, pc := range physicalCoreList {
		if pc < cpu.physicalCoreMin {
			cpu.physicalCoreMin = pc
		}
		if pc > cpu.physicalCoreMax {
			cpu.physicalCoreMax = pc
		}
	}
	for _, lc := range logicalCoreList {
		if lc < cpu.logicalCoreMin {
			cpu.logicalCoreMin = lc
		}
		if lc > cpu.logicalCoreMax {
			cpu.logicalCoreMax = lc
		}
	}
	for i := 0; i < len(physicalCoreList); i++ {
		cpu.corePairs[i] = CorePair{
			PhysicalCore: physicalCoreList[i],
			LogicalCore:  logicalCoreList[i],
		}
	}

	return &cpu, nil
}

func (c CPU) isValidCorePair(pair CorePair) bool {
	if pair.PhysicalCore < c.physicalCoreMin || pair.PhysicalCore > c.physicalCoreMax {
		return false
	}
	if pair.LogicalCore < c.logicalCoreMin || pair.LogicalCore > c.logicalCoreMax {
		return false
	}
	return true
}

// AvailableCorePair is
func (c CPU) AvailableCorePair() int {
	return len(c.corePairs)
}

// PopCorePair is
func (c *CPU) PopCorePair() (*CorePair, error) {
	if len(c.corePairs) < 1 {
		return nil, ErrNoValidCoreFound
	}
	pair := c.corePairs[0]
	c.corePairs = c.corePairs[1:]
	return &pair, nil
}

// PushCorePair is
func (c *CPU) PushCorePair(pair CorePair) error {
	if !c.isValidCorePair(pair) {
		return ErrInvalidCorePair
	}
	c.corePairs = append([]CorePair{pair}, c.corePairs...)
	return nil
}

func extractCoreID(s string) (coreIDs []int, err error) {
	s = strings.TrimSpace(s)
	num := strings.Split(s, "-")
	if len(num) < 2 {
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, err
		}
		return []int{int(n)}, nil
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
		coreIDs = append(coreIDs, int(i))
	}

	return coreIDs, nil
}
