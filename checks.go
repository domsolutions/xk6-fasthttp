package fasthttp

import (
	"errors"
	"strconv"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules/k6"
	"go.k6.io/k6/metrics"
)

func (mi *ModuleInstance) CheckStatus(wantStatus int, r *sobek.Object, extras ...sobek.Value) (bool, error) {
	state := mi.vu.State()
	if state == nil {
		return false, k6.ErrCheckInInitContext
	}

	if r == nil {
		return false, errors.New("nil response")
	}
	resp, ok := r.Export().(*Response)
	if !ok {
		return false, errors.New("response object not given to CheckStatus")
	}

	ctx := mi.vu.Context()
	rt := mi.vu.Runtime()
	t := time.Now()

	// Prepare the metric tags
	commonTagsAndMeta := state.Tags.GetCurrentValues()
	if len(extras) > 0 {
		if err := common.ApplyCustomUserTags(rt, &commonTagsAndMeta, extras[0]); err != nil {
			return false, err
		}
	}

	checkName := "check status is " + strconv.FormatInt(int64(wantStatus), 10)

	tags := commonTagsAndMeta.Tags
	if state.Options.SystemTags.Has(metrics.TagCheck) {
		tags = tags.With("check", checkName)
	}

	pass := resp.Status == wantStatus

	sample := metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.Checks,
			Tags:   tags,
		},
		Time:     t,
		Metadata: commonTagsAndMeta.Metadata,
		Value:    0,
	}

	metrics.PushIfNotDone(ctx, state.Samples, sample)

	return pass, nil
}
