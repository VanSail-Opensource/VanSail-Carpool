package xnode

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"

	"google.golang.org/protobuf/encoding/protowire"
)

type geoByteCounter struct {
	*bufio.Reader
	n int64
}

func (r *geoByteCounter) ReadByte() (byte, error) {
	b, err := r.Reader.ReadByte()
	if err == nil {
		r.n++
	}
	return b, err
}

// geoRecords uses ReaderAt to scan a country twice without retaining its tree.
// Production already supplies a staged file; other readers use one bounded spool.
func geoRecords(ctx context.Context, input io.Reader, visit func(io.ReaderAt, int64, int64) error) error {
	at, ok := input.(io.ReaderAt)
	if !ok {
		file, err := os.CreateTemp("", "xnode-geoip-decode-*")
		if err != nil {
			return assetStorageFailure()
		}
		defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
		n, err := io.Copy(file, io.LimitReader(input, maxGeoIPAsset+1))
		if err != nil {
			return assetTransferFailure("GeoIP asset", err)
		}
		if n > maxGeoIPAsset {
			return ErrInvalid
		}
		at = file
	}
	for offset := int64(0); ; {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := &geoByteCounter{Reader: bufio.NewReader(io.NewSectionReader(at, offset, maxGeoIPAsset-offset))}
		tag, err := binary.ReadUvarint(r)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil || tag != 10 {
			return ErrInvalid
		}
		size, err := binary.ReadUvarint(r)
		if err != nil || size == 0 || size > uint64(maxGeoIPAsset-offset-r.n) {
			return ErrInvalid
		}
		offset += r.n
		if err = visit(at, offset, int64(size)); err != nil {
			return err
		}
		offset += int64(size)
	}
}

// walkGeoCountry retains only the requested scalar or a single CIDR message.
func walkGeoCountry(ctx context.Context, input io.Reader, field protowire.Number, visit func(protowire.Number, protowire.Type, []byte) error) error {
	r := bufio.NewReader(input)
	for count := 0; ; count++ {
		if count%256 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		tag, err := binary.ReadUvarint(r)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil || tag>>3 == 0 {
			return ErrInvalid
		}
		number, kind := protowire.Number(tag>>3), protowire.Type(tag&7)
		var raw []byte
		switch kind {
		case protowire.BytesType:
			size, err := binary.ReadUvarint(r)
			if err != nil || size > maxGeoIPAsset {
				return ErrInvalid
			}
			wanted := number == field || field == 1 && number == 1
			if !wanted {
				if _, err = io.CopyN(io.Discard, r, int64(size)); err != nil {
					return ErrInvalid
				}
				continue
			}
			limit := uint64(512)
			if field == 1 {
				limit = 64
			}
			if size > limit {
				return ErrInvalid
			}
			raw = make([]byte, int(size))
			if _, err = io.ReadFull(r, raw); err != nil {
				return ErrInvalid
			}
		case protowire.VarintType:
			value, err := binary.ReadUvarint(r)
			if err != nil {
				return ErrInvalid
			}
			raw = protowire.AppendVarint(nil, value)
		case protowire.Fixed32Type, protowire.Fixed64Type:
			size := int64(4)
			if kind == protowire.Fixed64Type {
				size = 8
			}
			if _, err = io.CopyN(io.Discard, r, size); err != nil {
				return ErrInvalid
			}
			continue
		default:
			return ErrInvalid
		}
		if err = visit(number, kind, raw); err != nil {
			return err
		}
	}
}
