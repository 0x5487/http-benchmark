package gateway

import (
	"hash/fnv"
	"http-benchmark/pkg/config"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobin(t *testing.T) {
	proxyOptions1 := ProxyOptions{
		Target:   "http://backend1",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy1, _ := NewReverseProxy(proxyOptions1, nil)

	proxyOptions2 := ProxyOptions{
		Target:   "http://backend2",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy2, _ := NewReverseProxy(proxyOptions2, nil)

	proxyOptions3 := ProxyOptions{
		Target:   "http://backend3",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy3, _ := NewReverseProxy(proxyOptions3, nil)

	upstream := &Upstream{
		proxies: []*Proxy{
			proxy1,
			proxy2,
			proxy3,
		},
		counter: atomic.Uint64{},
	}

	expected := []string{"http://backend1", "http://backend2", "http://backend3"}
	for _, e := range expected {
		proxy := upstream.roundRobin()
		assert.NotNil(t, proxy)
		assert.Equal(t, e, proxy.target)
	}
}

func TestWeighted(t *testing.T) {
	proxyOptions1 := ProxyOptions{
		Target:   "http://backend1",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy1, _ := NewReverseProxy(proxyOptions1, nil)

	proxyOptions2 := ProxyOptions{
		Target:   "http://backend2",
		Protocol: config.ProtocolHTTP,
		Weight:   2,
	}
	proxy2, _ := NewReverseProxy(proxyOptions2, nil)

	proxyOptions3 := ProxyOptions{
		Target:   "http://backend3",
		Protocol: config.ProtocolHTTP,
		Weight:   3,
	}
	proxy3, _ := NewReverseProxy(proxyOptions3, nil)

	upstream := &Upstream{
		proxies: []*Proxy{
			proxy1,
			proxy2,
			proxy3,
		},
		totalWeight: 6,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	hits := map[string]int{"http://backend1": 0, "http://backend2": 0, "http://backend3": 0}
	for i := 0; i < 6000; i++ {
		proxy := upstream.weighted()
		assert.NotNil(t, proxy)
		hits[proxy.target]++
	}

	// Assert that weight distribution is roughly correct
	assert.InDelta(t, 1000, hits["http://backend1"], 100)
	assert.InDelta(t, 2000, hits["http://backend2"], 100)
	assert.InDelta(t, 3000, hits["http://backend3"], 100)
}

func TestRandom(t *testing.T) {
	proxyOptions1 := ProxyOptions{
		Target:   "http://backend1",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy1, _ := NewReverseProxy(proxyOptions1, nil)

	proxyOptions2 := ProxyOptions{
		Target:   "http://backend2",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy2, _ := NewReverseProxy(proxyOptions2, nil)

	proxyOptions3 := ProxyOptions{
		Target:   "http://backend3",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy3, _ := NewReverseProxy(proxyOptions3, nil)

	upstream := &Upstream{
		proxies: []*Proxy{
			proxy1,
			proxy2,
			proxy3,
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	hits := map[string]int{"http://backend1": 0, "http://backend2": 0, "http://backend3": 0}
	for i := 0; i < 10000; i++ {
		proxy := upstream.random()
		assert.NotNil(t, proxy)
		hits[proxy.target]++
	}

	// Assert that each proxy was selected roughly equally
	assert.InDelta(t, 3333, hits["http://backend1"], 500)
	assert.InDelta(t, 3333, hits["http://backend2"], 500)
	assert.InDelta(t, 3333, hits["http://backend3"], 500)
}

func TestHashing(t *testing.T) {
	proxyOptions1 := ProxyOptions{
		Target:   "http://backend1",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy1, _ := NewReverseProxy(proxyOptions1, nil)

	proxyOptions2 := ProxyOptions{
		Target:   "http://backend2",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy2, _ := NewReverseProxy(proxyOptions2, nil)

	proxyOptions3 := ProxyOptions{
		Target:   "http://backend3",
		Protocol: config.ProtocolHTTP,
		Weight:   1,
	}
	proxy3, _ := NewReverseProxy(proxyOptions3, nil)

	upstream := &Upstream{
		proxies: []*Proxy{
			proxy1,
			proxy2,
			proxy3,
		},
		hasher: fnv.New32a(),
	}

	keys := []string{"key1", "key2", "key3"}
	expected := map[string]string{
		"key1": "http://backend3",
		"key2": "http://backend1",
		"key3": "http://backend1",
	}

	for _, key := range keys {
		proxy := upstream.hasing(key)
		assert.NotNil(t, proxy)
		assert.Equal(t, expected[key], proxy.target)
	}
}
