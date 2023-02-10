package stat

import (
	"log"
	"sync/atomic"
)

type Stat struct {
	putCount         int64
	getCount         int64
	getForValidCount int64
	listCount        int64
	deleteCount      int64
}

func (st *Stat) AddPutCount() {
	atomic.AddInt64(&st.putCount, 1)
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
	log.Println("Statistics report.")
	log.Printf("put count: %d\n", st.putCount)
	log.Printf("get count: %d\n", st.getCount)
	log.Printf("get (for validation) count: %d\n", st.getForValidCount)
	log.Printf("list count: %d\n", st.listCount)
	log.Printf("delete count: %d\n", st.deleteCount)
}
