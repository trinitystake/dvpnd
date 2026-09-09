// SPDX-License-Identifier: Apache-2.0

package bandwidth

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/showwin/speedtest-go/speedtest"
)

// maxConnections is how many parallel streams a throughput test opens. The
// library's default is the CPU count, and a two-core host cannot fill a
// gigabit link at 20 ms with two streams.
const maxConnections = 8

// ooklaProber measures against speedtest.net servers.
type ooklaProber struct {
	client  *speedtest.Speedtest
	servers map[string]*speedtest.Server
}

func newOoklaProber(opts Options) *ooklaProber {
	conf := &speedtest.UserConfig{
		PingMode:       speedtest.HTTP, // the path the transfers use; ICMP is often filtered
		MaxConnections: maxConnections,
	}
	if opts.Latitude != 0 || opts.Longitude != 0 {
		// Sent to the service as the point to list servers around, so the list
		// does not depend on its own record of this address.
		conf.Location = &speedtest.Location{Lat: opts.Latitude, Lon: opts.Longitude}
	}

	// The client carries no Timeout: it would cover the whole body read and
	// cut the transfer tests short. Every call is bounded by its context.
	//
	// WithDoer must come before WithUserConfig: the latter wires the library's
	// transport onto whatever client is set at that moment. (The library also
	// rewrites http.DefaultClient's transport when the package loads; the node
	// never uses that client, its geolocation lookups build their own.)
	client := speedtest.New(
		speedtest.WithDoer(&http.Client{}),
		speedtest.WithUserConfig(conf),
	)

	return &ooklaProber{client: client, servers: map[string]*speedtest.Server{}}
}

// Candidates fetches the server list. The library pings every server once,
// in parallel, under its own four-second ceiling, so each candidate arrives
// with a first latency sample, or a non-positive one when unreachable. The
// user-info lookup is deliberately not made: its only effect would be a
// distance figure derived from the service's record of this address.
func (p *ooklaProber) Candidates(ctx context.Context) ([]Candidate, error) {
	list, err := p.client.FetchServerListContext(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(list))
	for _, s := range list {
		p.servers[s.ID] = s
		out = append(out, candidateOf(s))
	}

	return out, nil
}

// Refine pings the servers accurately, ten echoes each, all in parallel. The
// ping writes only the server's own fields, so this is safe; the transfer
// tests are not, see Measure.
func (p *ooklaProber) Refine(ctx context.Context, candidates []Candidate) []Candidate {
	out := make([]Candidate, len(candidates))

	var wg sync.WaitGroup
	for i, c := range candidates {
		s, ok := p.servers[c.ID]
		if !ok {
			out[i] = c
			out[i].Latency = -1
			continue
		}

		wg.Add(1)
		go func(i int, s *speedtest.Server) {
			defer wg.Done()
			c := candidateOf(s)
			if err := s.PingTestContext(ctx, nil); err != nil {
				c.Latency = -1
			} else {
				c = candidateOf(s)
			}
			out[i] = c
		}(i, s)
	}
	wg.Wait()

	return out
}

// Measure runs the download then the upload test. The two share the client's
// data manager, so servers are measured one at a time and the manager is reset
// between them.
func (p *ooklaProber) Measure(ctx context.Context, c Candidate) (Sample, error) {
	s, ok := p.servers[c.ID]
	if !ok {
		return Sample{}, fmt.Errorf("unknown server %q", c.ID)
	}

	p.client.Reset()
	if err := s.DownloadTestContext(ctx); err != nil {
		return Sample{}, err
	}
	p.client.Wait()

	if err := s.UploadTestContext(ctx); err != nil {
		return Sample{}, err
	}
	p.client.Wait()

	// ByteRate is bytes per second; -1 means the library could not measure.
	return Sample{Candidate: c, Upload: float64(s.ULSpeed), Download: float64(s.DLSpeed)}, nil
}

func candidateOf(s *speedtest.Server) Candidate {
	return Candidate{
		ID:         s.ID,
		Name:       s.Name,
		Sponsor:    s.Sponsor,
		Country:    s.Country,
		Host:       s.Host,
		Latency:    s.Latency,
		MinLatency: s.MinLatency,
	}
}
