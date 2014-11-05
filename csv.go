package csv

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

type Config struct {
	// When true, leading and trailing spaces are trimmed form
	// unquoted fields.
	TrimSpaces bool
	// Byte that separates fields in a row. Usually ','.
	FieldDelim byte
}

// The default config. Most CSV should use this.
// Based on RFC 4180 (http://http://tools.ietf.org/html/rfc4180).
func DefaultConfig() Config {
	return Config{TrimSpaces: false, FieldDelim: ','}
}

type Reader struct {
	tmpbuf bytes.Buffer
	br     io.ByteReader
	Config Config
}

// Creates a reader with the default Config.
func NewReader(r io.ByteReader) *Reader {
	return &Reader{br: r, Config: DefaultConfig()}
}

func (r *Reader) parseQuoted() (string, byte, error) {
	r.tmpbuf.Reset()
	for {
		b, e := r.br.ReadByte()
		if e != nil {
			if e == io.EOF {
				e = io.ErrUnexpectedEOF
			}
			return "", 0, e
		}

		if b == '"' {
			b, e = r.br.ReadByte()
			if b == '"' && e == nil {
				// if we got two double-quotes, parse as one
				r.tmpbuf.WriteByte('"')
			} else {
				// eat trailing whitespace
				for b == ' ' && e == nil {
					b, e = r.br.ReadByte()
				}
				return r.tmpbuf.String(), b, nil
			}
		} else {
			// anything not a quote is just copied over
			r.tmpbuf.WriteByte(b)
		}
	}
	panic("unreachable")
}

func (r *Reader) parseCell() (string, byte, error) {
	r.tmpbuf.Reset()
	b, e := r.br.ReadByte()
	if r.Config.TrimSpaces {
		for b == ' ' && e == nil {
			// eat leading whitespace
			b, e = r.br.ReadByte()
		}
	}
	if e == io.EOF {
		return "", 0, e
	}
	if b == '"' && e == nil {
		return r.parseQuoted()
	}
	trailing_spaces := 0
	var last byte
	for e == nil && b != '\n' && b != r.Config.FieldDelim {
		if r.Config.TrimSpaces {
			if b == ' ' || b == '\r' {
				trailing_spaces += 1
			} else {
				trailing_spaces = 0
			}
		}
		r.tmpbuf.WriteByte(b)
		last = b
		b, e = r.br.ReadByte()
	}
	if e != nil && e != io.EOF {
		return "", 0, e
	}
	if last == '\r' && b == '\n' && trailing_spaces == 0 {
		trailing_spaces = 1
	}
	s := r.tmpbuf.Bytes()
	return string(s[0 : len(s)-trailing_spaces]), b, nil
}

// Reads a single row into a []string.
func (r *Reader) ReadRow() ([]string, error) {
	var result []string
	for {
		c, b, e := r.parseCell()
		if e != nil {
			if e == io.EOF && len(result) > 0 {
				result = append(result, c)
			}
			return result, e
		}
		result = append(result, c)
		if b == 0 {
			break
		}
		// Line endings may be '\r\n', so eat '\r'.
		if b == '\r' {
			b, e = r.br.ReadByte()
			if e != nil {
				return nil, e
			}
		}
		if b == r.Config.FieldDelim {
			continue
		} else if b == '\n' {
			break
		} else {
			return nil, errors.New("expected , got " + string(int(b)))
		}
	}
	return result, nil
}

func (r *Reader) ReadAll() ([][]string, error) {
	rows := make([][]string, 0, 32)
	for {
		row, e := r.ReadRow()
		if e != nil {
			if e == io.EOF {
				break
			}
			return nil, e
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// Convenience function that reads the whole CSV file into memory and returns it as
// [][]string (a slice of rows, which are a slice of strings).
func ReadAll(r io.Reader) ([][]string, error) {
	return NewReader(bufio.NewReader(r)).ReadAll()
}

type Writer struct {
	out    *bufio.Writer
	Config Config
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{bufio.NewWriter(w), DefaultConfig()}
}

func (w *Writer) needsQuotes(s string) bool {
	if len(s) > 0 {
		if s[0] == ' ' || s[len(s)-1] == ' ' {
			return true
		}
	}
	for _, c := range s {
		switch c {
		case '\n', '"', '\t', rune(w.Config.FieldDelim):
			return true
		}
	}
	return false
}

func (w *Writer) writeCell(cell string) (e error) {
	if w.needsQuotes(cell) {
		e = w.out.WriteByte('"')
		if e != nil {
			return
		}
		for i := 0; i < len(cell); i++ {
			b := cell[i]
			if b == '"' {
				_, e = w.out.WriteString(`""`)
				if e != nil {
					return
				}
			} else {
				e = w.out.WriteByte(b)
				if e != nil {
					return
				}
			}
		}
		e = w.out.WriteByte('"')
		if e != nil {
			return
		}
	} else {
		_, e = w.out.WriteString(cell)
	}
	return
}

func (w *Writer) WriteRow(row []string) (e error) {
	for i, cell := range row {
		if i > 0 {
			e = w.out.WriteByte(w.Config.FieldDelim)
			if e != nil {
				return
			}
		}
		e = w.writeCell(cell)
		if e != nil {
			return
		}
	}
	e = w.out.WriteByte('\n')
	if e != nil {
		return
	}
	e = w.out.Flush()
	return
}

func (w *Writer) WriteAll(rows [][]string) error {
	for _, row := range rows {
		if e := w.WriteRow(row); e != nil {
			return e
		}
	}
	return nil
}

// Convenience function to write a [][]string as CSV
// with the default Config.
func WriteAll(out io.Writer, rows [][]string) error {
	return NewWriter(out).WriteAll(rows)
}
