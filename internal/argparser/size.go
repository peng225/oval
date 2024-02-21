package argparser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func ParseSize(s string) (int, int, error) {
	s = strings.ToLower(s)
	var sizeStrs []string
	if strings.Contains(s, "-") {
		sizeStrs = strings.Split(s, "-")
		if len(sizeStrs) != 2 {
			return 0, 0, fmt.Errorf("illegal size format: %v", s)
		}
	} else {
		sizeStrs = []string{s, s}
	}

	sizes := make([]int, 2)
	for i, sizeStr := range sizeStrs {
		var err error
		sizes[i], err = parseSizeUnit(sizeStr)
		if err != nil {
			return 0, 0, err
		}
	}
	if sizes[0] > sizes[1] {
		return 0, 0, errors.New("maxSize should be larger than minSize")
	}
	return sizes[0], sizes[1], nil
}

func parseSizeUnit(s string) (int, error) {
	unit := map[string]int{
		"k": 1024,
		"m": 1024 * 1024,
		"g": 1024 * 1024 * 1024,
	}
	r := regexp.MustCompile("^[1-9][0-9]*[kmg]*$")
	if r.MatchString(s) {
		if size, err := strconv.Atoi(s); err == nil {
			return size, nil
		} else {
			baseNum, err := strconv.Atoi(s[:len(s)-1])
			if err != nil {
				return 0, err
			}
			return baseNum * unit[s[len(s)-1:]], nil
		}
	}
	return 0, fmt.Errorf("illegal size format: %v", s)
}

func ParseMultipartThresh(s string) (int, error) {
	mpThresh, err := parseSizeUnit(s)
	return mpThresh, err
}
