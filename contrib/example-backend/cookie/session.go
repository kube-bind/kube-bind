package cookie

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/pierrec/lz4"
	"github.com/vmihailenco/msgpack/v4"
)

type SessionState struct {
	CreatedAt time.Time `msgpack:"ca,omitempty"`
	ExpiresOn time.Time `msgpack:"eo,omitempty"`

	AccessToken  string `msgpack:"at,omitempty"`
	IDToken      string `msgpack:"it,omitempty"`
	RefreshToken string `msgpack:"rt,omitempty"`

	RedirectURL string `msgpack:"ru,omitempty"`
	SessionID   string `msgpack:"si,omitempty"`
}

func (s *SessionState) Encode() ([]byte, error) {
	return msgpack.Marshal(s)
}

func Decode(data string) (*SessionState, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	var ss SessionState
	err = msgpack.Unmarshal(decoded, &ss)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling data to session state: %w", err)
	}

	return &ss, nil
}

// lz4Compress compresses with LZ4
//
// The Compress:Decompress ratio is 1:Many. LZ4 gives fastest decompress speeds
// at the expense of greater compression compared to other compression
// algorithms.
func lz4Compress(payload []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := lz4.NewWriter(nil)
	zw.Header = lz4.Header{
		BlockMaxSize:     65536,
		CompressionLevel: 0,
	}
	zw.Reset(buf)

	reader := bytes.NewReader(payload)
	_, err := io.Copy(zw, reader)
	if err != nil {
		return nil, fmt.Errorf("error copying lz4 stream to buffer: %w", err)
	}
	err = zw.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing lz4 writer: %w", err)
	}

	compressed, err := io.ReadAll(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading lz4 buffer: %w", err)
	}

	return compressed, nil
}

// lz4Decompress decompresses with LZ4
func lz4Decompress(compressed []byte) ([]byte, error) {
	reader := bytes.NewReader(compressed)
	buf := new(bytes.Buffer)
	zr := lz4.NewReader(nil)
	zr.Reset(reader)
	_, err := io.Copy(buf, zr)
	if err != nil {
		return nil, fmt.Errorf("error copying lz4 stream to buffer: %w", err)
	}

	payload, err := io.ReadAll(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading lz4 buffer: %w", err)
	}

	return payload, nil
}
