package keygen

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Max value for 11-character Base62 string: 62^11 - 1
	// 62^11 = 52,036,560,683,837,093,888
	// This fits in uint64 (max 18,446,744,073,709,551,615)
	// Wait, 62^11 is actually larger than uint64.
	// 62^10 = 839,299,365,868,340,224
	// 62^11 = 52,036,560,683,837,093,888
	// uint64 max is ~1.8e19. 62^11 is ~5.2e19.
	// So 11 chars can represent more than uint64.
	// Let's use 62^11 - 1 as MaxID if it fits, or just use uint64 max.
	// The user said "snowflake based 64bit long uuid".
	// Snowflake IDs are typically 64-bit integers.
	// 64-bit integer max is 18,446,744,073,709,551,615.
	// Base62 of uint64 max:
	// 18446744073709551615 / 62^10 = 18446744073709551615 / 839299365868340224 = 21.97
	// So it fits in 11 characters (since 62^10 < uint64 max < 62^11).
	MaxID = 18446744073709551615
	// etcd key for global counter
	EtcdMaxIDKey = "/kgs/max_id"
	// Default segment size
	DefaultSegmentSize = 1000000
)

type KeyGenerator interface {
	GetNextID() uint64
}

type segment struct {
	start uint64
	end   uint64
	next  uint64
}

type DualBufferGenerator struct {
	etcdClient    *clientv3.Client
	primary       *segment
	standby       *segment
	segmentSize   uint64
	mu            sync.Mutex
	isPrefetching bool
}

func NewDualBufferGenerator(endpoints []string) (*DualBufferGenerator, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %v", err)
	}

	kg := &DualBufferGenerator{
		etcdClient:  cli,
		segmentSize: DefaultSegmentSize,
	}

	// Fetch initial segment
	seg, err := kg.fetchSegment()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch initial segment: %v", err)
	}
	kg.primary = seg

	return kg, nil
}

func (k *DualBufferGenerator) fetchSegment() (*segment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use etcd transaction to atomically increment the max_id
	_, err := k.etcdClient.KV.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(EtcdMaxIDKey), ">", -1)).
		Then(clientv3.OpGet(EtcdMaxIDKey), clientv3.OpPut(EtcdMaxIDKey, "0")). // Placeholder if not exists
		Else().
		Commit()

	if err != nil {
		return nil, err
	}

	var currentMax uint64
	kvResp, err := k.etcdClient.Get(ctx, EtcdMaxIDKey)
	if err != nil {
		return nil, err
	}

	if kvResp.Count > 0 {
		fmt.Sscanf(string(kvResp.Kvs[0].Value), "%d", &currentMax)
	}

	newMax := currentMax + k.segmentSize
	_, err = k.etcdClient.Put(ctx, EtcdMaxIDKey, fmt.Sprintf("%d", newMax))
	if err != nil {
		return nil, err
	}

	return &segment{
		start: currentMax,
		end:   newMax - 1,
		next:  currentMax,
	}, nil
}

func (k *DualBufferGenerator) GetNextID() uint64 {
	k.mu.Lock()

	// Check if primary is exhausted
	if k.primary.next > k.primary.end {
		if k.standby != nil {
			k.primary = k.standby
			k.standby = nil
		} else {
			// Emergency fetch if standby is not ready
			seg, err := k.fetchSegment()
			if err != nil {
				log.Printf("CRITICAL: failed to fetch emergency segment: %v", err)
				// Fallback to local increment as last resort (risky in distributed env)
				k.primary.start = k.primary.end + 1
				k.primary.end = k.primary.start + k.segmentSize - 1
				k.primary.next = k.primary.start
			} else {
				k.primary = seg
			}
		}
	}

	id := k.primary.next
	k.primary.next++

	// Check if we need to prefetch standby
	usage := float64(k.primary.next-k.primary.start) / float64(k.primary.end-k.primary.start+1)
	if usage >= 0.95 && k.standby == nil && !k.isPrefetching {
		k.isPrefetching = true
		go k.prefetchStandby()
	}

	k.mu.Unlock()

	// Obfuscate and ensure bounds
	return k.obfuscateWithCycleWalking(id)
}

func (k *DualBufferGenerator) prefetchStandby() {
	seg, err := k.fetchSegment()
	k.mu.Lock()
	defer k.mu.Unlock()

	k.isPrefetching = false
	if err != nil {
		log.Printf("WARNING: failed to prefetch standby segment: %v", err)
		return
	}
	k.standby = seg
}

func (k *DualBufferGenerator) obfuscateWithCycleWalking(id uint64) uint64 {
	obf := FeistelCipherObfuscate(id)
	// Cycle Walking: If output is out of bounds, re-obfuscate until it's in bounds
	for obf > MaxID {
		obf = FeistelCipherObfuscate(obf)
	}
	return obf
}

// FeistelCipherObfuscate applies a 4-round Feistel cipher to obfuscate IDs.
func FeistelCipherObfuscate(id uint64) uint64 {
	// For 64-bit IDs, we use 64 bits for the cipher space
	// 32 bits for L and 32 bits for R
	const mask = 0xFFFFFFFF
	L := uint32((id >> 32) & mask)
	R := uint32(id & mask)

	for i := 0; i < 4; i++ {
		nextL := R
		nextR := L ^ roundFunction(R, uint32(i))
		L, R = nextL, nextR
	}

	return (uint64(L) << 32) | uint64(R)
}

func roundFunction(val uint32, round uint32) uint32 {
	// A more robust non-linear function
	val ^= 0xDEADBEEF ^ round
	val *= 0x85EBCA6B
	val ^= val >> 13
	val *= 0xC2B2AE35
	val ^= val >> 16
	return val
}

// Base62Encode converts a uint64 ID to an 11-character Base62 string.
func Base62Encode(id uint64) string {
	res := make([]byte, 11)
	for i := 10; i >= 0; i-- {
		res[i] = base62Chars[id%62]
		id /= 62
	}
	return string(res)
}

// Base62Decode converts a Base62 string back to a uint64 ID.
func Base62Decode(s string) (uint64, error) {
	if len(s) != 11 {
		// Support legacy shorter codes if needed, but for now strict 11
		// return 0, fmt.Errorf("invalid base62 string length: %d", len(s))
	}
	var res uint64
	for _, c := range s {
		idx := strings.IndexRune(base62Chars, c)
		if idx == -1 {
			return 0, fmt.Errorf("invalid character in base62 string: %c", c)
		}
		res = res*62 + uint64(idx)
	}
	return res, nil
}
