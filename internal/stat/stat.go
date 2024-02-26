package stat

import (
	"log/slog"
	"sync/atomic"
)

type Stat struct {
	putCount          int64
	uploadedPartCount int64
	getCount          int64
	getForValidCount  int64
	listCount         int64
	deleteCount       int64
}

func (st *Stat) AddPutCount() {
	atomic.AddInt64(&st.putCount, 1)
}

func (st *Stat) AddUploadedPartCount(partCount int64) {
	atomic.AddInt64(&st.uploadedPartCount, partCount)
}

func (st *Stat) AddGetCount() {
	atomic.AddInt64(&st.getCount, 1)
}

func (st *Stat) AddGetForValidCount() {
	atomic.AddInt64(&st.getForValidCount, 1)
}

func (st *Stat) AddListCount() {
	atomic.AddInt64(&st.listCount, 1)
}

func (st *Stat) AddDeleteCount() {
	atomic.AddInt64(&st.deleteCount, 1)
}

func (st *Stat) Report() {
	slog.Info("Statistics report.",
		slog.Group("report", "putCount", st.putCount,
			"numUploadedParts", st.uploadedPartCount,
			"getCount", st.getCount,
			"getForValidationCount", st.getForValidCount,
			"listCount", st.listCount,
			"deleteCount", st.deleteCount,
		),
	)
}
