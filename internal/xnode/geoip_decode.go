package xnode

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"go4.org/netipx"
	"google.golang.org/protobuf/encoding/protowire"
	"io"
	"strings"
)

func readGeoIPPrefixes(ctx context.Context, input io.Reader, selectors []string) ([]string, error) {
	selected := map[string]bool{}
	for _, selector := range selectors {
		selected[strings.TrimPrefix(selector, "geoip:")] = false
	}
	reader := bufio.NewReader(input)
	var builder netipx.IPSetBuilder
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tag, err := binary.ReadUvarint(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || tag != 10 {
			return nil, ErrInvalid
		}
		size, err := binary.ReadUvarint(reader)
		if err != nil || size == 0 || size > 16<<20 {
			return nil, ErrInvalid
		}
		raw := make([]byte, int(size))
		if _, err = io.ReadFull(reader, raw); err != nil {
			return nil, ErrInvalid
		}
		code, reverse, err := geoCountryHeader(raw)
		if err != nil {
			return nil, err
		}
		code = strings.ToLower(code)
		if _, wanted := selected[code]; !wanted {
			continue
		}
		if reverse {
			return nil, ErrInvalid
		}
		selected[code] = true
		err = geoFields(raw, func(number protowire.Number, kind protowire.Type, value []byte) error {
			if number != 2 {
				return nil
			}
			if kind != protowire.BytesType {
				return ErrInvalid
			}
			count++
			if count > 500000 {
				return ErrInvalid
			}
			if count%256 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			prefix, err := geoCIDR(value)
			if err != nil {
				return err
			}
			builder.AddPrefix(prefix)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, found := range selected {
		if !found {
			return nil, ErrInvalid
		}
	}
	set, err := builder.IPSet()
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, prefix := range set.Prefixes() {
		result = append(result, prefix.String())
	}
	return result, nil
}
