// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/matryer/is"
)

func TestParseAuthTokenFile(t *testing.T) {
	is := is.New(t)

	f, err := os.CreateTemp("", "*.json")
	is.NoErr(err)

	fmt.Fprintf(f, "{\"token1\": \"foo\", \"token2\": \"bar\", \"token3\": \"baz\"}")
	f.Seek(0, io.SeekStart)

	tokens, err := parseAuthTokensFile(f.Name())
	is.NoErr(err)

	is.Equal(len(tokens), 3)
	is.Equal(tokens["token1"], "foo")
	is.Equal(tokens["token2"], "bar")
	is.Equal(tokens["token3"], "baz")
}

func TestSetEnvironmentFromFileJSON(t *testing.T) {
	is := is.New(t)

	f, err := os.CreateTemp("", "*.json")
	is.NoErr(err)

	fmt.Fprintf(f, "{\"var-number-1\": \"value1\", \"var_number_TWO\": \"value2\"}")
	f.Seek(0, io.SeekStart)

	for _, prefix := range []string{"", "PREFIX_"} {
		t.Run("using prefix "+prefix, func(t *testing.T) {
			is := is.New(t)

			t.Setenv("VAR_NUMBER_1", "")
			t.Setenv("VAR_NUMBER_TWO", "")

			err = setEnvironmentFromJSONFile(prefix, f.Name())
			is.NoErr(err)
			is.Equal(os.Getenv(prefix+"VAR_NUMBER_1"), "value1")
			is.Equal(os.Getenv(prefix+"VAR_NUMBER_TWO"), "value2")
		})
	}
}
