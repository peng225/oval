package argparser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/peng225/oval/runner"
)

func OpeRatioParse(opeRatioStr string) ([]float64, error) {
	opeRatioStrs := strings.Split(opeRatioStr, ",")
	if len(opeRatioStrs) != int(runner.NumOperation) {
		return nil, fmt.Errorf("invalid ope ratio format %v", opeRatioStr)
	}

	ratios := make([]float64, int(runner.NumOperation))
	sum := 0.0
	for i, v := range opeRatioStrs {
		intV, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		ratios[i] = float64(intV)
		sum += ratios[i]
	}
	for i, _ := range ratios {
		ratios[i] /= sum
	}
	return ratios, nil
}
