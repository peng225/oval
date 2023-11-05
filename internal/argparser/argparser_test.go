package argparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMultipartThresh(t *testing.T) {
	type testCase struct {
		multipartThreshStr      string
		expectedMultipartThresh int
		expectedErr             bool
	}
	testCases := []testCase{
		{
			multipartThreshStr:      "512",
			expectedMultipartThresh: 512,
			expectedErr:             false,
		},
		{
			multipartThreshStr:      "2k",
			expectedMultipartThresh: 2048,
			expectedErr:             false,
		},
		{
			multipartThreshStr:      "4m",
			expectedMultipartThresh: 4 * 1024 * 1024,
			expectedErr:             false,
		},
		{
			multipartThreshStr:      "12m",
			expectedMultipartThresh: 12 * 1024 * 1024,
			expectedErr:             false,
		},
		{
			multipartThreshStr:      "8g",
			expectedMultipartThresh: 8 * 1024 * 1024 * 1024,
			expectedErr:             false,
		},
		{
			multipartThreshStr:      "5t",
			expectedMultipartThresh: 0,
			expectedErr:             true,
		},
	}

	for _, tc := range testCases {
		multipartThresh, err := ParseMultipartThresh(tc.multipartThreshStr)
		if tc.expectedErr {
			assert.Errorf(t, err, "tc.multipartThreshStr: %s", tc.multipartThreshStr)
			continue
		}
		assert.Equal(t, tc.expectedMultipartThresh, multipartThresh)
	}
}
