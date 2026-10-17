package xnode

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"google.golang.org/protobuf/encoding/protowire"
	"io"
	"strings"
)

func readRoutingGeoSite(ctx context.Context, input io.Reader, selectors []string) (map[string][]string, error) {
	result := map[string][]string{}
	wanted := map[string]string{}
	for _, s := range selectors {
		code, _, _ := strings.Cut(strings.TrimPrefix(s, "geosite:"), "@")
		wanted[s] = strings.ToUpper(code)
	}
	r := bufio.NewReader(input)
	count := 0
	expandedCount, expandedBytes := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tag, err := binary.ReadUvarint(r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || tag != 10 {
			return nil, ErrInvalid
		}
		size, err := binary.ReadUvarint(r)
		if err != nil || size == 0 || size > 32<<20 {
			return nil, ErrInvalid
		}
		raw := make([]byte, int(size))
		if _, err = io.ReadFull(r, raw); err != nil {
			return nil, err
		}
		code, _, err := geoCountryHeader(raw)
		if err != nil {
			return nil, err
		}
		selected := []string{}
		for s, c := range wanted {
			if strings.EqualFold(c, code) {
				selected = append(selected, s)
				if result[s] == nil {
					result[s] = []string{}
				}
			}
		}
		if len(selected) == 0 {
			continue
		}
		err = geoFields(raw, func(n protowire.Number, k protowire.Type, value []byte) error {
			if n != 2 {
				return nil
			}
			if k != protowire.BytesType || len(value) > 16384 {
				return ErrInvalid
			}
			count++
			if count > 100000 {
				return ErrInvalid
			}
			if count%256 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			kind, domain, attrs, err := routingGeoDomain(value)
			if err != nil {
				return err
			}
			prefix := []string{"keyword:", "regexp:", "domain:", "full:"}
			if kind >= uint64(len(prefix)) {
				return ErrInvalid
			}
			for _, selector := range selected {
				_, attribute, has := strings.Cut(selector, "@")
				match := true
				if has {
					for _, a := range strings.Split(attribute, "@") {
						match = match && attrs[strings.ToLower(a)]
					}
				}
				if match {
					expandedCount++
					expandedBytes += len(prefix[kind]) + len(domain)
					if expandedCount > 100000 || expandedBytes > 8<<20 {
						return ErrInvalid
					}
					result[selector] = append(result[selector], prefix[kind]+domain)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, s := range selectors {
		if len(result[s]) == 0 {
			return nil, ErrInvalid
		}
	}
	return result, nil
}
func routingGeoDomain(raw []byte) (kind uint64, domain string, attrs map[string]bool, err error) {
	attrs = map[string]bool{}
	err = geoFields(raw, func(n protowire.Number, k protowire.Type, v []byte) error {
		switch n {
		case 1:
			if k != protowire.VarintType {
				return ErrInvalid
			}
			value, size := protowire.ConsumeVarint(v)
			if size < 0 {
				return ErrInvalid
			}
			kind = value
		case 2:
			if k != protowire.BytesType || len(v) == 0 || len(v) > 2048 {
				return ErrInvalid
			}
			domain = string(v)
		case 3:
			if k != protowire.BytesType {
				return ErrInvalid
			}
			return geoFields(v, func(n protowire.Number, k protowire.Type, v []byte) error {
				if n == 1 {
					if k != protowire.BytesType || len(v) > 128 {
						return ErrInvalid
					}
					attrs[strings.ToLower(string(v))] = true
				}
				return nil
			})
		}
		return nil
	})
	if domain == "" && err == nil {
		err = ErrInvalid
	}
	return
}
