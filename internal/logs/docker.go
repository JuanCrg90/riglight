package logs

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
)

// ConsumeDocker parses Docker's fixed eight-byte multiplex protocol without
// allocating from the peer-controlled declared frame length.
func ConsumeDocker(ctx context.Context, body io.Reader, tty bool, emit Emit) error {
	reader := NewBudgetReader(body)
	sink := NewLineSink(emit)
	if tty {
		return consumeDockerPlain(ctx, reader, sink)
	}
	header := make([]byte, 8)
	chunk := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := readFull(reader, header); err != nil {
			if errors.Is(err, io.EOF) {
				return sink.Close()
			}
			return dockerReadError(ctx, err)
		}
		if header[1] != 0 || header[2] != 0 || header[3] != 0 || (header[0] != 1 && header[0] != 2) {
			return ErrInvalid
		}
		stream := StreamStdout
		if header[0] == 2 {
			stream = StreamStderr
		}
		left := int64(binary.BigEndian.Uint32(header[4:]))
		for left > 0 {
			count := int64(len(chunk))
			if left < count {
				count = left
			}
			if err := readFull(reader, chunk[:count]); err != nil {
				return dockerReadError(ctx, err)
			}
			if err := sink.Write(stream, chunk[:count]); err != nil {
				return err
			}
			left -= count
		}
	}
}

func consumeDockerPlain(ctx context.Context, reader io.Reader, sink *lineSink) error {
	chunk := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := reader.Read(chunk)
		if count > 0 {
			if writeErr := sink.Write(StreamOutput, chunk[:count]); writeErr != nil {
				return writeErr
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return sink.Close()
		}
		return dockerReadError(ctx, err)
	}
}
func readFull(reader io.Reader, data []byte) error {
	_, err := io.ReadFull(reader, data)
	return err
}
func dockerReadError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, ErrByteLimit) {
		return ErrByteLimit
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return ErrInvalid
	}
	if errors.Is(err, io.EOF) {
		return io.EOF
	}
	return ErrRead
}
