package validator

type Validator struct {
	NumObj     int64
	NumWorker  int
	MinSize    int
	MaxSize    int
	NumRound   int
	BucketName string
}

func (v *Validator) Run() {

}
