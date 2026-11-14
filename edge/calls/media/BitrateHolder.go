package media

import (
	"sync"

	"github.com/pion/interceptor/pkg/cc"
)

type BitrateHolder struct {
	lock      sync.Mutex
	estimator cc.BandwidthEstimator
	fallback  int
}

func NewBitrateHolder(fallback int) *BitrateHolder {
	return &BitrateHolder{fallback: fallback}
}

func (holder *BitrateHolder) Set(id string, estimator cc.BandwidthEstimator) {
	holder.lock.Lock()
	defer holder.lock.Unlock()
	holder.estimator = estimator
}

func (holder *BitrateHolder) Bitrate() int {
	holder.lock.Lock()
	defer holder.lock.Unlock()
	return holder.estimated()
}

func (holder *BitrateHolder) estimated() int {
	if holder.estimator == nil {
		return holder.fallback
	}
	return holder.estimator.GetTargetBitrate()
}
