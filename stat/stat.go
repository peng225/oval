package stat

import (
	"fmt"
	"sync/atomic"
)

type Stat struct {
	putCount         int64
	getCount         int64
	getForValidCount int64
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

func (st *Stat) AddDeleteCount() {
	atomic.AddInt64(&st.deleteCount, 1)
}

func (st *Stat) Report() {
	fmt.Println("Statistics report.")
	fmt.Printf("put count: %d\n", st.putCount)
	fmt.Printf("get count: %d\n", st.getCount)
	fmt.Printf("get (for validation) count: %d\n", st.getForValidCount)
	fmt.Printf("delete count: %d\n", st.deleteCount)
}
