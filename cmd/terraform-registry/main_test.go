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
