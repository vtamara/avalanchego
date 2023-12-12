// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package version

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseApplication(t *testing.T) {
	v, err := ParseApplication("avalanche/1.2.3")

	require.NoError(t, err)
	require.NotNil(t, v)
	require.Equal(t, "avalanche/1.2.3", v.String())
	require.Equal(t, 1, v.Major)
	require.Equal(t, 2, v.Minor)
	require.Equal(t, 3, v.Patch)
	require.NoError(t, v.Compatible(v))
	require.False(t, v.Before(v))

	tests := []struct {
		version     string
		expectedErr error
	}{
		{
			version:     "",
			expectedErr: errMissingApplicationPrefix,
		},
		{
			version:     "avalanche/",
			expectedErr: errMissingVersions,
		},
		{
			version:     "avalanche/z.0.0",
			expectedErr: strconv.ErrSyntax,
		},
		{
			version:     "avalanche/0.z.0",
			expectedErr: strconv.ErrSyntax,
		},
		{
			version:     "avalanche/0.0.z",
			expectedErr: strconv.ErrSyntax,
		},
		{
			version:     "avalanche/0.0.0.0",
			expectedErr: strconv.ErrSyntax,
		},
	}
	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			_, err := ParseApplication(test.version)
			require.ErrorIs(t, err, test.expectedErr)
		})
	}
}
