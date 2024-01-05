package tracer

import (
	"net"
	"time"

	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

// A Trail represents detailed information about an HTTP request.
// You'd typically get one from a Tracer.
type Trail struct {
	EndTime time.Time

	// Total connect time (Connecting + TLSHandshaking)
	ConnDuration time.Duration

	// Total request duration, excluding DNS lookup and connect time.
	Duration time.Duration

	ConnRemoteAddr net.Addr

	Failed null.Bool
	// Populated by SaveSamples()
	Tags     *metrics.TagSet
	Metadata map[string]string
	Samples  []metrics.Sample
}

// SaveSamples populates the Trail's sample slice so they're accesible via GetSamples()
func (tr *Trail) SaveSamples(builtinMetrics *metrics.BuiltinMetrics, ctm *metrics.TagsAndMeta) {
	tr.Tags = ctm.Tags
	tr.Metadata = ctm.Metadata
	tr.Samples = make([]metrics.Sample, 0, 2) // this is with 1 more for a possible HTTPReqFailed
	tr.Samples = append(tr.Samples, []metrics.Sample{
		{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.HTTPReqs,
				Tags:   ctm.Tags,
			},
			Time:     tr.EndTime,
			Metadata: ctm.Metadata,
			Value:    1,
		},
		{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.HTTPReqDuration,
				Tags:   ctm.Tags,
			},
			Time:     tr.EndTime,
			Metadata: ctm.Metadata,
			Value:    metrics.D(tr.Duration),
		},
	}...)
}

// GetSamples implements the metrics.SampleContainer interface.
func (tr *Trail) GetSamples() []metrics.Sample {
	return tr.Samples
}

// GetTags implements the metrics.ConnectedSampleContainer interface.
func (tr *Trail) GetTags() *metrics.TagSet {
	return tr.Tags
}

// GetTime implements the metrics.ConnectedSampleContainer interface.
func (tr *Trail) GetTime() time.Time {
	return tr.EndTime
}

// Ensure that interfaces are implemented correctly
var _ metrics.ConnectedSampleContainer = &Trail{}
