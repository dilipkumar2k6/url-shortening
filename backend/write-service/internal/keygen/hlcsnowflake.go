package keygen

import (
	"net"
	"sync/atomic"
	"time"
)

const (
	epoch          = uint64(1767225600000) // Jan 1, 2026
	nodeIdBits     = 10
	sequenceBits   = 12
	maxNodeId      = (1 << nodeIdBits) - 1
	maxSequence    = (1 << sequenceBits) - 1
	nodeIdShift    = sequenceBits
	timestampShift = sequenceBits + nodeIdBits
)

type HLCSnowflakeGenerator struct {
	nodeId      uint64
	logicalTime atomic.Uint64
	sequence    atomic.Uint64
}

func NewHLCSnowflakeGenerator() *HLCSnowflakeGenerator {
	return &HLCSnowflakeGenerator{
		nodeId: getNodeIdFromIp() & maxNodeId,
	}
}

func (s *HLCSnowflakeGenerator) physicalTimeMillis() uint64 {
	return uint64(time.Now().UnixMilli())
}

func (s *HLCSnowflakeGenerator) GetNextID() uint64 {
	physicalTime := s.physicalTimeMillis()
	currentLogicalTime := s.logicalTime.Load()

	var newLogicalTime uint64

	if physicalTime > currentLogicalTime {
		newLogicalTime = physicalTime
		s.logicalTime.Store(newLogicalTime)
		s.sequence.Store(0)
	} else {
		newLogicalTime = currentLogicalTime
		seq := (s.sequence.Add(1)) & maxSequence
		if seq == 0 {
			newLogicalTime = s.logicalTime.Add(1)
		}
	}

	id := ((newLogicalTime - epoch) << timestampShift) |
		(s.nodeId << nodeIdShift) |
		s.sequence.Load()

	return id
}

func getNodeIdFromIp() uint64 {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return 0
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.To4()
				return uint64(ip[2])<<8 | uint64(ip[3])
			}
		}
	}
	return 0
}
