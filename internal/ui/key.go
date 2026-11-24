package ui

import (
	"bufio"
	"io"
	"unicode/utf8"
)

// Code identifies a keypress that is not a printable character.
type Code int

const (
	Char Code = iota
	Enter
	Tab
	Backspace
	Delete
	Left
	Right
	Up
	Down
	Home
	End
	CtrlA
	CtrlC
	CtrlD
	CtrlE
	CtrlK
	CtrlL
	CtrlU
	CtrlW
	Esc
	Unknown
)

// Key is one decoded keypress.
type Key struct {
	Code Code
	R    rune
}

type keyReader struct {
	r    *bufio.Reader
	back []Key
}

func newKeyReader(r io.Reader) *keyReader {
	return &keyReader{r: bufio.NewReaderSize(r, 256)}
}

func (k *keyReader) Read() (Key, error) {
	if n := len(k.back); n > 0 {
		key := k.back[n-1]
		k.back = k.back[:n-1]
		return key, nil
	}
	return k.read()
}

func (k *keyReader) unread(key Key) { k.back = append(k.back, key) }

func (k *keyReader) read() (Key, error) {
	b, err := k.r.ReadByte()
	if err != nil {
		return Key{}, err
	}
	switch b {
	case '\r', '\n':
		return Key{Code: Enter}, nil
	case '\t':
		return Key{Code: Tab}, nil
	case 0x7f, 0x08:
		return Key{Code: Backspace}, nil
	case 0x01:
		return Key{Code: CtrlA}, nil
	case 0x03:
		return Key{Code: CtrlC}, nil
	case 0x04:
		return Key{Code: CtrlD}, nil
	case 0x05:
		return Key{Code: CtrlE}, nil
	case 0x0b:
		return Key{Code: CtrlK}, nil
	case 0x0c:
		return Key{Code: CtrlL}, nil
	case 0x15:
		return Key{Code: CtrlU}, nil
	case 0x17:
		return Key{Code: CtrlW}, nil
	case 0x1b:
		return k.escape()
	}
	if b < 0x20 {
		return Key{Code: Unknown}, nil
	}
	if b < utf8.RuneSelf {
		return Key{Code: Char, R: rune(b)}, nil
	}
	_ = k.r.UnreadByte()
	r, _, err := k.r.ReadRune()
	if err != nil {
		return Key{}, err
	}
	return Key{Code: Char, R: r}, nil
}

// escape decodes the arrow/home/end/delete sequences every console sends.
// A bare ESC is reported as Esc and the byte that followed it is pushed back,
// so a terminal in application-cursor mode still works.
func (k *keyReader) escape() (Key, error) {
	b1, err := k.r.ReadByte()
	if err != nil {
		return Key{Code: Esc}, nil
	}
	if b1 != '[' && b1 != 'O' {
		_ = k.r.UnreadByte()
		return Key{Code: Esc}, nil
	}
	b2, err := k.r.ReadByte()
	if err != nil {
		return Key{Code: Esc}, nil
	}
	switch b2 {
	case 'A':
		return Key{Code: Up}, nil
	case 'B':
		return Key{Code: Down}, nil
	case 'C':
		return Key{Code: Right}, nil
	case 'D':
		return Key{Code: Left}, nil
	case 'H':
		return Key{Code: Home}, nil
	case 'F':
		return Key{Code: End}, nil
	}
	if b2 >= '0' && b2 <= '9' {
		digits := []byte{b2}
		for len(digits) < 8 {
			nb, err := k.r.ReadByte()
			if err != nil || nb == '~' {
				break
			}
			digits = append(digits, nb)
		}
		switch string(digits) {
		case "1", "7":
			return Key{Code: Home}, nil
		case "4", "8":
			return Key{Code: End}, nil
		case "3":
			return Key{Code: Delete}, nil
		}
	}
	return Key{Code: Unknown}, nil
}
