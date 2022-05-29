package validator

import (
	"fmt"
	"sync"
)

type Validator struct {
	NumObj     int64
	NumWorker  int
	MinSize    int
	MaxSize    int
	NumRound   int
	BucketName string
}

func (v *Validator) Run() {
	for round := 0; round < v.NumRound; round++ {
		fmt.Printf("round: %v\n", round)
		// Create phase
		wg := &sync.WaitGroup{}
		for i := 0; i < v.NumWorker; i++ {
			wg.Add(1)
			go func() {
				v.create()
				wg.Done()
			}()
		}
		wg.Wait()

		// Update phase
		for i := 0; i < v.NumWorker; i++ {
			wg.Add(1)
			go func() {
				v.update()
				wg.Done()
			}()
		}
		wg.Wait()

		// Delete phase
		for i := 0; i < v.NumWorker; i++ {
			wg.Add(1)
			go func() {
				v.delete()
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func (v *Validator) create() {
	// TODO: implement
}

func (v *Validator) update() {
	// TODO: implement
}

func (v *Validator) delete() {
	// TODO: implement
}
