package stat

import "fmt"

type Stat struct {
	putCount    int64
	deleteCount int64
}

func (st *Stat) AddPutCount() {
	st.putCount++
}

func (st *Stat) AddDeleteCount() {
	st.deleteCount++
}

func (st *Stat) Report() {
	fmt.Println("Statistics report.")
	fmt.Printf("put count: %d\n", st.putCount)
	fmt.Printf("delete count: %d\n", st.deleteCount)
}
