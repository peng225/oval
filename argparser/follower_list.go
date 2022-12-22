package argparser

import (
	"fmt"
	"strings"
)

func ParseFollowerList(followerListStr string) ([]string, error) {
	followerList := strings.Split(followerListStr, ",")
	if len(followerList) == 0 || followerList[0] == "" {
		return nil, fmt.Errorf("invalid follower list format %v", followerList)
	}
	return followerList, nil
}
