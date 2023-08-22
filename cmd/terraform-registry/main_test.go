// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestParseAuthTokenFile(t *testing.T) {
	is := is.New(t)

	tokens, err := parseAuthTokens([]byte(`{"token1": "foo", "token2": "bar", "token3": "baz"}`))
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

func TestWatchFile(t *testing.T) {
	is := is.New(t)

	f, err := os.CreateTemp(t.TempDir(), "*.json")
	is.NoErr(err)

	results := make(chan []byte, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	callback := func(b []byte) { results <- b }
	go watchFile(ctx, f.Name(), 50*time.Millisecond, callback)
	is.Equal(<-results, []byte{}) // initial callback before we've written anything

	_, err = f.WriteString("foo")
	is.NoErr(err)
	is.Equal(<-results, []byte("foo")) // after first write

	_, err = f.WriteString("bar")
	is.NoErr(err)
	is.Equal(<-results, []byte("foobar")) // after second write

	time.Sleep(100 * time.Millisecond)
	is.Equal(len(results), 0) // should not be any more events
}
