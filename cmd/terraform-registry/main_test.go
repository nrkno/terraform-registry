// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/matryer/is"
)

func TestParseAuthTokenFileNewLine(t *testing.T) {
	is := is.New(t)

	f, err := os.CreateTemp("", "")
	is.NoErr(err)

	fmt.Fprintf(f, "foo\nbar\n\n\n\nbaz\n")
	f.Seek(0, io.SeekStart)

	tokens, err := parseAuthTokensFile(f.Name())
	is.NoErr(err)

	is.Equal(len(tokens), 3)
	is.Equal(tokens[0], "foo")
	is.Equal(tokens[1], "bar")
	is.Equal(tokens[2], "baz")
}

func TestParseAuthTokenFileJSON(t *testing.T) {
	is := is.New(t)

	f, err := os.CreateTemp("", "*.json")
	is.NoErr(err)

	fmt.Fprintf(f, "{\"token1\": \"foo\", \"token2\": \"bar\", \"token3\": \"baz\"}")
	f.Seek(0, io.SeekStart)

	tokens, err := parseAuthTokensFile(f.Name())
	is.NoErr(err)
	sort.Strings(tokens)

	is.Equal(len(tokens), 3)
	is.Equal(tokens[0], "bar")
	is.Equal(tokens[1], "baz")
	is.Equal(tokens[2], "foo")
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
