package argparser

import (
	"fmt"
	"slices"
)

func ValidateFollowerList(followerList []string) error {
	if len(followerList) == 0 || slices.Contains(followerList, "") {
		return fmt.Errorf("invalid follower list format %v", followerList)
	}
	return nil
}
