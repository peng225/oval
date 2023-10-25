package argparser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/peng225/oval/runner"
)

func ParseOpeRatio(opeRatioStr string) ([]float64, error) {
	opeRatioStrs := strings.Split(opeRatioStr, ",")
	if len(opeRatioStrs) != int(runner.NumOperation) {
		return nil, fmt.Errorf("invalid ope ratio format %v", opeRatioStr)
	}

	ratio := make([]float64, int(runner.NumOperation))
	sum := 0.0
	for i, v := range opeRatioStrs {
		intV, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		ratio[i] = float64(intV)
		sum += ratio[i]
	}
	for i := range ratio {
		ratio[i] /= sum
	}
	return ratio, nil
}
