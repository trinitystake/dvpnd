// SPDX-License-Identifier: Apache-2.0

package common

import (
	"sync"
)

// PeerSet is the set of peer keys a proxy service has registered with its
// child process, keyed by the session key (base64 of the peer data).
type PeerSet struct {
	mu sync.RWMutex
	m  map[string]struct{}
}

func NewPeerSet() *PeerSet {
	return &PeerSet{m: make(map[string]struct{})}
}

func (p *PeerSet) Put(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[key] = struct{}{}
}

func (p *PeerSet) Delete(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, key)
}

func (p *PeerSet) Has(key string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.m[key]

	return ok
}

func (p *PeerSet) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.m)
}

// Keys returns a snapshot of the registered keys.
func (p *PeerSet) Keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keys := make([]string, 0, len(p.m))
	for k := range p.m {
		keys = append(keys, k)
	}

	return keys
}
