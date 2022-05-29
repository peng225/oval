package argparser

import (
	"strconv"
	"strings"
)

func SizeParse(s string) (int, int, error) {
	s = strings.ToLower(s)
	var sizesStr []string
	if strings.Contains(s, "-") {
		sizesStr = strings.Split(s, "-")
		if len(sizesStr) != 2 {
			panic("len(sizesStr) != 2")
		}
	} else {
		sizesStr = []string{s}
	}

	sizes := make([]int, 2)
	for i, sizeStr := range sizesStr {
		sizeStrByte, err := parseSizeUnit(sizeStr)
		if err != nil {
			return 0, 0, err
		}
		sizes[i], err = strconv.Atoi(sizeStrByte)
		if err != nil {
			return 0, 0, err
		}
	}
	return sizes[0], sizes[1], nil
}

func parseSizeUnit(s string) (string, error) {
	// TODO: implement
	return s, nil
}
