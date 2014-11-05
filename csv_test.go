package csv

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

type testHelper struct {
	*testing.T
}

func fmtError(s string, args ...interface{}) error {
	return fmt.Errorf(s, args...)
}

func (t testHelper) checkNoErr(e error) {
	t.checkThat(e, NotError())
}

func NotError() Matcher {
	return func(actual interface{}) error {
		if actual != nil {
			return fmtError("Expected no error, got %q", actual)
		}
		return nil
	}
}

type Matcher func(interface{}) error

func (t testHelper) checkThat(actual interface{}, matcher Matcher) bool {
	if e := matcher(actual); e != nil {
		t.Error(e.Error())
		return false
	}
	return true
}

func IsOneOf(items ...interface{}) Matcher {
	return func(v interface{}) error {
		for _, i := range items {
			if reflect.DeepEqual(i, v) {
				return nil
			}
		}
		return fmtError("%v was not one of %v", v, items)
	}
}

func Equals(i interface{}) Matcher {
	return func(v interface{}) error {
		if reflect.DeepEqual(i, v) {
			return nil
		}
		return fmtError("Expected %#v, got %#v", i, v)
	}
}

func (t testHelper) checkEq(actual, expected interface{}) bool {
	return t.checkThat(actual, Equals(expected))
}

func str2Reader(s string) *Reader {
	return NewReader(strings.NewReader(s))
}

func TestParseCell(tp *testing.T) {
	t := testHelper{tp}

	var cases = []struct {
		in, expected string
	}{
		{"   meh ", "   meh "},
		{"hi", "hi"},
		{"1 2 3 ", "1 2 3 "},
		{"oh,", "oh"},
		{"oh\nno", "oh"},
		{`"Hi, mom"`, "Hi, mom"},
		{`"Hi, ""mr"" silly"`, `Hi, "mr" silly`},
		{"\"Whee\"  ,", "Whee"},
	}
	for _, tc := range cases {
		p := str2Reader(tc.in)
		c, _, e := p.parseCell()
		t.checkThat(e, NotError())
		t.checkEq(c, tc.expected)
	}
}

func TestParseCellErr(tp *testing.T) {
	t := testHelper{tp}
	p := str2Reader(`"Unterminated`)
	s, _, e := p.parseCell()
	t.checkEq(e, io.ErrUnexpectedEOF)
	t.checkEq(s, "")
}

func TestParseCellEof(tp *testing.T) {
	t := testHelper{tp}
	p := str2Reader("")
	_, _, e := p.parseCell()
	t.checkEq(e, io.EOF)
}

func TestReadRow(tp *testing.T) {
	t := testHelper{tp}
	var cases = []struct {
		in       string
		expected []string
		trim     bool
	}{
		{"one,two,three", []string{"one", "two", "three"}, false},
		{" one,   two ,three    \n", []string{"one", "two", "three"}, true},
		{",,", []string{"", "", ""}, true},
		{`"foo ",bar`, []string{"foo ", "bar"}, false},
		{"", nil, true},
		{" ", nil, true},
	}
	for _, tc := range cases {
		p := str2Reader(tc.in)
		p.Config.TrimSpaces = tc.trim
		r, e := p.ReadRow()
		t.checkThat(e, IsOneOf(nil, io.EOF))
		t.checkEq(r, tc.expected)
	}
}

func TestReadRowMultiple(tp *testing.T) {
	t := testHelper{tp}
	str := "a,\"b\"\n\"c\"  ,d"
	p := str2Reader(str)

	r, e := p.ReadRow()
	t.checkNoErr(e)
	t.checkEq(r, []string{"a", "b"})

	r, e = p.ReadRow()
	t.checkNoErr(e)
	t.checkEq(r, []string{"c", "d"})

	r, e = p.ReadRow()
	t.checkEq(e, io.EOF)
	t.checkEq(r, []string(nil))
}

func TestReadRowEof(tp *testing.T) {
	t := testHelper{tp}
	p := str2Reader("one,two\n")

	p.ReadRow()
	r, e := p.ReadRow()
	t.checkEq(e, io.EOF)
	t.checkEq(r, []string(nil))
}

func TestReadRowNoTrim(tp *testing.T) {
	t := testHelper{tp}
	p := str2Reader("  a  ,  b  ,\" c \" ")
	p.Config.TrimSpaces = false
	r, e := p.ReadRow()
	t.checkNoErr(e)
	t.checkEq(r, []string{"  a  ", "  b  ", " c "})
}

func TestReadAllNoTrim(tp *testing.T) {
	t := testHelper{tp}
	p := str2Reader("  a  ,  b  \r\n  c,  d ")
	rows, e := p.ReadAll()
	t.checkNoErr(e)
	t.checkEq(rows, [][]string{
		{"  a  ", "  b  "},
		{"  c", "  d "}})
}

func TestReadAllTrim(tp *testing.T) {
	t := testHelper{tp}
	str := `whee,foo,bar
 one,two,three,four
"foo ", bar, what
meh,beh, keh
`

	p := str2Reader(str)
	p.Config.TrimSpaces = true
	rows, e := p.ReadAll()
	t.checkNoErr(e)
	t.checkEq(len(rows), 4)
	t.checkEq(rows[0], []string{"whee", "foo", "bar"})
	t.checkEq(rows[1], []string{"one", "two", "three", "four"})
	t.checkEq(rows[2], []string{"foo ", "bar", "what"})
	t.checkEq(rows[3], []string{"meh", "beh", "keh"})
}

func TestReadAllLineEnding(tp *testing.T) {
	t := testHelper{tp}
	str := "one,two\r\nthree,\"four\"\r\n5,6"
	rows, e := ReadAll(strings.NewReader(str))
	t.checkNoErr(e)
	t.checkEq(rows, [][]string{
		{"one", "two"},
		{"three", "four"},
		{"5", "6"}})
}

func TestReadAllEmpty(tp *testing.T) {
	t := testHelper{tp}
	str := ""
	rows, e := ReadAll(strings.NewReader(str))
	t.checkNoErr(e)
	t.checkEq(len(rows), 0)
}

func TestWriteLeadingSpace(tp *testing.T) {
	t := testHelper{tp}
	out := bytes.NewBuffer(nil)
	data := [][]string{
		{" A ", " "},
		{"B", "C"}}
	t.checkNoErr(WriteAll(out, data))
	expected := `" A "," "` + "\nB,C\n"
	t.checkEq(out.String(), expected)
}

func TestSemiDelim(tp *testing.T) {
	t := testHelper{tp}
	p := str2Reader("1;2;3\n4;5;6")
	config := DefaultConfig()
	config.FieldDelim = ';'
	p.Config = config
	rs, e := p.ReadAll()
	t.checkNoErr(e)
	t.checkEq(rs, [][]string{{"1", "2", "3"}, {"4", "5", "6"}})

	out := bytes.NewBuffer(nil)
	w := NewWriter(out)
	w.Config = config
	w.WriteAll(rs)
	t.checkEq(out.String(), "1;2;3\n4;5;6\n")
}

func BenchmarkParsing(b *testing.B) {
	b.StopTimer()
	str := strings.Repeat("aaaaaaaa,b b b b b b b,\"fo \n oo\",\"c oh c yes c \", ddddd ddd\n", 2000)
	b.SetBytes(int64(len(str)))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		in := strings.NewReader(str)
		rows, e := ReadAll(in)
		if e != nil {
			panic(e)
		} else if len(rows) != 2000 {
			panic("wrong # rows")
		}
	}
}
