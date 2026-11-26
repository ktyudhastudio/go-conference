package services

import (
	"time"

	"github.com/pion/webrtc/v4"

	"go-conference/core"
)

// qualityInterval is how often a publisher's connection is sampled (var so tests can speed it up).
var qualityInterval = 3 * time.Second

// Thresholds for grading a connection (publisher → server direction).
const (
	fairRTTms, poorRTTms       = 200, 400
	fairLossPct, poorLossPct   = 3.0, 8.0
	fairJitterMs, poorJitterMs = 30, 60
)

// gradeQuality maps raw metrics to good / fair / poor (worst metric wins).
func gradeQuality(rttMs int, lossPct float64, jitterMs int) string {
	switch {
	case rttMs > poorRTTms || lossPct > poorLossPct || jitterMs > poorJitterMs:
		return core.QualityPoor
	case rttMs > fairRTTms || lossPct > fairLossPct || jitterMs > fairJitterMs:
		return core.QualityFair
	}
	return core.QualityGood
}

type counters struct {
	received uint64
	lost     int64
	bytes    uint64
}

// monitorQuality samples pc until it closes and reports the grade of the media the
// participant is sending to us (RTT, loss, jitter, bitrate).
func monitorQuality(pc *webrtc.PeerConnection, report func(core.Quality)) {
	go func() {
		t := time.NewTicker(qualityInterval)
		defer t.Stop()
		var prev counters
		var prevAt time.Time
		for range t.C {
			if pc.ConnectionState() == webrtc.PeerConnectionStateClosed {
				return
			}
			var cur counters
			var rttMs, jitterMs int
			for _, st := range pc.GetStats() {
				switch s := st.(type) {
				case webrtc.InboundRTPStreamStats:
					cur.received += uint64(s.PacketsReceived)
					cur.lost += int64(s.PacketsLost)
					cur.bytes += s.BytesReceived
					if j := int(s.Jitter * 1000); j > jitterMs {
						jitterMs = j
					}
				case webrtc.ICECandidatePairStats:
					if s.Nominated && s.CurrentRoundTripTime > 0 {
						rttMs = int(s.CurrentRoundTripTime * 1000)
					}
				}
			}

			now := time.Now()
			if prevAt.IsZero() { // first sample only sets the baseline
				prev, prevAt = cur, now
				continue
			}
			dRecv := cur.received - prev.received
			dLost := cur.lost - prev.lost
			var lossPct float64
			if total := int64(dRecv) + dLost; total > 0 && dLost > 0 {
				lossPct = float64(dLost) * 100 / float64(total)
			}
			var kbps int
			if secs := now.Sub(prevAt).Seconds(); secs > 0 && cur.bytes >= prev.bytes {
				kbps = int(float64(cur.bytes-prev.bytes) * 8 / 1000 / secs)
			}
			prev, prevAt = cur, now

			report(core.Quality{
				RTTMs: rttMs, LossPct: round1(lossPct), JitterMs: jitterMs, BitrateKbps: kbps,
				Level: gradeQuality(rttMs, lossPct, jitterMs),
			})
		}
	}()
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
