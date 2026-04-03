package player

import (
	"io"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
)

type readCloser struct {
	io.Reader
}

func (rc readCloser) Close() error { return nil }

func DecodeMP3(r io.Reader) (beep.StreamSeekCloser, beep.Format, error) {
	return mp3.Decode(readCloser{r})
}
