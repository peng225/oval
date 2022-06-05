package stat

import (
	"fmt"
	"sync/atomic"
)

type Stat struct {
	putCount    int64
	deleteCount int64
}

func (st *Stat) AddPutCount() {
	atomic.AddInt64(&st.putCount, 1)
}

func (st *Stat) AddDeleteCount() {
	atomic.AddInt64(&st.deleteCount, 1)
}

func (st *Stat) Report() {
	fmt.Println("Statistics report.")
	fmt.Printf("put count: %d\n", st.putCount)
	fmt.Printf("delete count: %d\n", st.deleteCount)
}
