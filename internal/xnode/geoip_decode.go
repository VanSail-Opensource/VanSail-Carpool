package xnode

import (
	"context"
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
	var builder netipx.IPSetBuilder
	count := 0
	err := geoRecords(ctx, input, func(at io.ReaderAt, offset, size int64) error {
		code, reverse := "", false
		err := walkGeoCountry(ctx, io.NewSectionReader(at, offset, size), 1, func(number protowire.Number, kind protowire.Type, value []byte) error {
			switch number {
			case 1:
				if kind != protowire.BytesType {
					return ErrInvalid
				}
				code = strings.ToLower(string(value))
			case 3:
				if kind != protowire.VarintType {
					return ErrInvalid
				}
				n, _ := protowire.ConsumeVarint(value)
				reverse = n != 0
			}
			return nil
		})
		if err != nil {
			return err
		}
		if _, wanted := selected[code]; !wanted {
			return nil
		}
		if reverse {
			return ErrInvalid
		}
		selected[code] = true
		return walkGeoCountry(ctx, io.NewSectionReader(at, offset, size), 2, func(number protowire.Number, kind protowire.Type, value []byte) error {
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
			prefix, err := geoCIDR(value)
			if err != nil {
				return err
			}
			builder.AddPrefix(prefix)
			return nil
		})
	})
	if err != nil {
		return nil, err
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
