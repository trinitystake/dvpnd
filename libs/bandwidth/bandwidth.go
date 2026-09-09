// SPDX-License-Identifier: Apache-2.0

// Package bandwidth establishes the bandwidth the node advertises: the
// operator's declared figure when the configuration carries one, otherwise a
// measurement of the internet link, and it says which of the two it is.
//
// The measurement is built to be right on any host without anyone tuning it.
// The pitfall it guards against is a speed-test server inside the node's own
// datacenter: a cloud host reaches such a server over the local network, so the
// test reports the local network's throughput, several times what the internet
// link can carry. Measured latency tells the two apart where the servers'
// claimed distances (derived from an IP geolocation database) do not, so
// servers are chosen by latency, the ones too close to be off this host's
// network are rejected, and the lowest of what independent servers achieve is
// reported. When in doubt the package understates rather than overstates:
// clients rank nodes by this figure and nothing verifies it.
package bandwidth

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Source values, reported on the status document next to the figure.
const (
	SourceSpeedtest = "speedtest" // measured against public speed-test servers
	SourceConfig    = "config"    // declared by the operator
	SourceNone      = "none"      // no usable measurement; the figure is zero
)

// bytesPerMegabit converts megabits per second to bytes per second. It is the
// constant the speed-test library divides by in ByteRate.Mbps(); the node
// applies no other conversion anywhere.
const bytesPerMegabit = 125000

const (
	// sameFacilityFloor is the lowest round-trip time a server may show and
	// still count as being off this host's network. A server in the same
	// facility answers in 0.1 to 1 ms, one in the same metropolitan area over
	// an exchange in 0.5 to 3 ms, and the two overlap; 2 ms (about 200 km of
	// fibre there and back) sits above both. Too low a floor lets the local
	// network through and overstates the link several times over; too high a
	// floor picks a slightly further server. The asymmetry decides it.
	sameFacilityFloor = 2 * time.Millisecond
	// sameBuildingFloor is the fallback when nothing clears sameFacilityFloor:
	// it still excludes the same rack and the same building, where the
	// measured figure is certainly the local network's.
	sameBuildingFloor = 1 * time.Millisecond

	refineBatch    = 10  // servers pinged accurately at a time, in parallel
	refineLimit    = 30  // servers pinged accurately at most
	wantQualifying = 3   // stop refining once this many clear the floor
	maxQualifying  = 5   // servers measured at most, counting failures
	maxSamples     = 3   // successful measurements at most
	agreementRatio = 0.7 // min/max between samples for them to agree
)

// Logger is the subset of the node's logger the package needs; nil means silent.
type Logger interface {
	Debug(msg string, keyVals ...interface{})
	Info(msg string, keyVals ...interface{})
	Error(msg string, keyVals ...interface{})
}

type nopLogger struct{}

func (nopLogger) Debug(string, ...interface{}) {}
func (nopLogger) Info(string, ...interface{})  {}
func (nopLogger) Error(string, ...interface{}) {}

// Options is what Measure needs from the node.
type Options struct {
	// UploadMbps and DownloadMbps, when both are positive, are the operator's
	// declared link; the measurement is skipped entirely.
	UploadMbps   float64
	DownloadMbps float64

	// Latitude and Longitude are where the node is, from the geolocation step.
	// They are sent to the speed-test service as the point to list servers
	// around, so the candidate list does not depend on that service's own
	// record of the node's address, which can be wrong.
	Latitude  float64
	Longitude float64

	// IP is the node's public address; it keys the cached measurement so that a
	// node moved to another host measures again.
	IP string

	// Home is the node's home directory, where the cached measurement lives.
	// Empty disables the cache.
	Home string

	Logger Logger
}

// Result is the figure the node advertises, in bytes per second, and where it
// came from.
type Result struct {
	Upload   int64
	Download int64
	Source   string
}

// BytesPerSecond converts a figure in megabits per second, the unit providers
// sell links in, to the bytes per second the node reports.
func BytesPerSecond(mbps float64) int64 {
	return int64(mbps * bytesPerMegabit)
}

// Mbps is the inverse of BytesPerSecond, for log lines.
func Mbps(bytesPerSecond float64) float64 {
	return bytesPerSecond / bytesPerMegabit
}

// Candidate is one speed-test server as the selection sees it.
type Candidate struct {
	ID      string
	Name    string // the city
	Sponsor string
	Country string
	Host    string

	// Latency is the mean round-trip time; zero or negative means unreachable.
	Latency time.Duration
	// MinLatency is the lowest round-trip time seen over the accurate ping;
	// zero until the candidate has been refined. The floor is applied to it,
	// because jitter can lift a mean but nothing lowers a minimum below the
	// distance light travels.
	MinLatency time.Duration
}

// String names the server the way an operator would recognise it in a log.
func (c Candidate) String() string {
	return fmt.Sprintf("%s (%s, %s)", c.Name, c.Sponsor, c.Country)
}

// Sample is one completed measurement, in bytes per second.
type Sample struct {
	Candidate Candidate
	Upload    float64
	Download  float64
}

// prober is the network side of the measurement. The selection and
// aggregation rules run against this interface, so the tests exercise them
// with a fake and never open a connection.
type prober interface {
	// Candidates lists the servers with one quick latency sample each.
	Candidates(ctx context.Context) ([]Candidate, error)
	// Refine pings the given servers accurately, in parallel, and returns
	// them with Latency and MinLatency set; a server that fails the ping comes
	// back with a non-positive Latency.
	Refine(ctx context.Context, candidates []Candidate) []Candidate
	// Measure runs the download and upload test against one server.
	Measure(ctx context.Context, c Candidate) (Sample, error)
}

// newProber builds the real prober; tests replace it.
var newProber = func(opts Options) prober {
	return newOoklaProber(opts)
}

// Measure returns the bandwidth to advertise. It never fails: whatever goes
// wrong is logged and shows in Result.Source, so a node always starts.
func Measure(ctx context.Context, opts Options) *Result {
	log := opts.Logger
	if log == nil {
		log = nopLogger{}
	}

	if opts.UploadMbps > 0 && opts.DownloadMbps > 0 {
		log.Info("Using the declared bandwidth", "upload_mbps", opts.UploadMbps, "download_mbps", opts.DownloadMbps)
		return &Result{
			Upload:   BytesPerSecond(opts.UploadMbps),
			Download: BytesPerSecond(opts.DownloadMbps),
			Source:   SourceConfig,
		}
	}

	now := time.Now()
	cached := readCache(opts.Home, log)
	if cached != nil && cached.IP == opts.IP && now.Sub(cached.MeasuredAt) < cacheTTL {
		log.Info("Reusing the link measurement", "measured_at", cached.MeasuredAt.Format(time.RFC3339),
			"age", now.Sub(cached.MeasuredAt).Round(time.Minute).String())
		return cached.result()
	}

	log.Info("Measuring the internet link", "ip", opts.IP, "latitude", opts.Latitude, "longitude", opts.Longitude)
	res := measure(ctx, newProber(opts), log)
	if res.Source == SourceSpeedtest {
		writeCache(opts.Home, opts.IP, res, now, log)
		return res
	}

	if cached != nil && cached.IP == opts.IP {
		log.Error("Fresh measurement failed; using the figure measured earlier",
			"measured_at", cached.MeasuredAt.Format(time.RFC3339))
		return cached.result()
	}

	log.Error("No usable bandwidth measurement; the node advertises zero until one succeeds. " +
		"To declare the link instead, set download_mbps and upload_mbps in the [bandwidth] section.")
	return res
}

// measure is the whole algorithm over a prober.
func measure(ctx context.Context, p prober, log Logger) *Result {
	none := &Result{Source: SourceNone}

	candidates, err := p.Candidates(ctx)
	if err != nil {
		log.Error("Could not fetch the speed test server list", "error", err)
		return none
	}

	reachable := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Latency > 0 {
			reachable = append(reachable, c)
		}
	}
	if len(reachable) == 0 {
		log.Error("No speed test server answered", "servers", len(candidates))
		return none
	}
	sortByLatency(reachable)
	log.Info("Speed test servers found", "reachable", len(reachable), "listed", len(candidates))

	// Refine the nearest servers' latency in batches until enough of them
	// clear the floor. In a dense market the ten nearest can all sit inside
	// the metropolitan area, and the next ten are the ones that count.
	var refined []Candidate
	limit := len(reachable)
	if limit > refineLimit {
		limit = refineLimit
	}
	for start := 0; start < limit; start += refineBatch {
		end := start + refineBatch
		if end > limit {
			end = limit
		}
		for _, c := range p.Refine(ctx, reachable[start:end]) {
			if c.Latency > 0 && c.MinLatency > 0 {
				refined = append(refined, c)
			}
		}
		if countClearing(refined, sameFacilityFloor) >= wantQualifying || ctx.Err() != nil {
			break
		}
	}
	sortByLatency(refined)
	for _, c := range refined {
		log.Debug("Speed test server considered", "server", c.String(), "latency", c.Latency, "min_latency", c.MinLatency)
	}

	list, rejected := shortlist(refined, sameFacilityFloor)
	for _, c := range rejected {
		log.Info("Speed test server rejected: too close to be off this host's network",
			"server", c.String(), "min_latency", c.MinLatency, "floor", sameFacilityFloor)
	}
	if len(list) == 0 {
		log.Error("Every reachable speed test server is within the floor; relaxing it. The figure may over-report.",
			"refined", len(refined), "floor", sameFacilityFloor, "relaxed_floor", sameBuildingFloor)
		list, _ = shortlist(refined, sameBuildingFloor)
		if len(list) == 0 {
			log.Error("Every reachable speed test server answers from inside this facility; " +
				"a measurement would report the local network, not the internet link, so none is made. " +
				"Declare the link with download_mbps and upload_mbps in the [bandwidth] section.")
			return none
		}
	}

	var samples []Sample
	for _, c := range list {
		if ctx.Err() != nil {
			log.Error("Speed test stopped", "error", ctx.Err())
			break
		}
		s, err := p.Measure(ctx, c)
		if err != nil {
			log.Error("Speed test failed", "server", c.String(), "error", err)
			continue
		}
		if s.Upload <= 0 || s.Download <= 0 {
			log.Error("Speed test returned no result", "server", c.String())
			continue
		}
		log.Info("Speed test result", "server", c.String(), "latency", c.Latency,
			"download_mbps", round1(Mbps(s.Download)), "upload_mbps", round1(Mbps(s.Upload)))
		samples = append(samples, s)
		if len(samples) >= maxSamples || agree(samples) {
			break
		}
	}
	if len(samples) == 0 {
		log.Error("Every speed test failed", "servers", len(list))
		return none
	}

	// The lowest independent server, per direction. Servers that clear the
	// latency floor can still be reached over the host's local peering, which
	// carries far more than the internet link the node actually sells to a
	// client anywhere; those read high, and the honest transit figure is the
	// low end. Taking the minimum never overstates. It can understate when the
	// lowest server is merely weak, which is the safe direction (clients rank
	// by this figure and nothing verifies it) and what the [bandwidth]
	// override is for.
	up, down := minSample(samples, func(s Sample) float64 { return s.Upload }),
		minSample(samples, func(s Sample) float64 { return s.Download })

	log.Info("Internet link measured", "download_mbps", round1(Mbps(down)), "upload_mbps", round1(Mbps(up)),
		"samples", len(samples))

	return &Result{Upload: int64(up), Download: int64(down), Source: SourceSpeedtest}
}

// shortlist picks, in order of mean latency, up to maxQualifying servers whose
// minimum latency clears the floor, one per sponsor and city so the paths are
// independent. It also returns the servers the floor rejected.
func shortlist(candidates []Candidate, floor time.Duration) (list, rejected []Candidate) {
	seen := map[string]bool{}
	for _, c := range candidates {
		if c.MinLatency < floor {
			rejected = append(rejected, c)
			continue
		}
		key := strings.ToLower(c.Sponsor + "|" + c.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		list = append(list, c)
		if len(list) == maxQualifying {
			break
		}
	}

	return list, rejected
}

func countClearing(candidates []Candidate, floor time.Duration) int {
	n := 0
	for _, c := range candidates {
		if c.MinLatency >= floor {
			n++
		}
	}

	return n
}

func sortByLatency(candidates []Candidate) {
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Latency < candidates[j].Latency })
}

// agree reports whether the samples are close enough, in both directions,
// that measuring another server would add nothing.
func agree(samples []Sample) bool {
	if len(samples) < 2 {
		return false
	}
	minUp, maxUp := samples[0].Upload, samples[0].Upload
	minDown, maxDown := samples[0].Download, samples[0].Download
	for _, s := range samples[1:] {
		minUp, maxUp = minF(minUp, s.Upload), maxF(maxUp, s.Upload)
		minDown, maxDown = minF(minDown, s.Download), maxF(maxDown, s.Download)
	}

	return minUp/maxUp >= agreementRatio && minDown/maxDown >= agreementRatio
}

// minSample is the smallest value of one direction across the samples.
func minSample(samples []Sample, pick func(Sample) float64) float64 {
	m := pick(samples[0])
	for _, s := range samples[1:] {
		if v := pick(s); v < m {
			m = v
		}
	}

	return m
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func round1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}
