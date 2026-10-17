package xnode

import (
	"google.golang.org/protobuf/encoding/protowire"
	"net/netip"
)

// Field numbers follow the pinned Xray common/geodata/geodat.proto. Walking
// fields before decoding prevents tiny repeated messages from amplifying memory.
func geoFields(raw []byte, visit func(protowire.Number, protowire.Type, []byte) error) error {
	for len(raw) > 0 {
		number, kind, tagBytes := protowire.ConsumeTag(raw)
		if tagBytes < 0 || kind == protowire.StartGroupType || kind == protowire.EndGroupType {
			return ErrInvalid
		}
		raw = raw[tagBytes:]
		size := protowire.ConsumeFieldValue(number, kind, raw)
		if size < 0 {
			return ErrInvalid
		}
		value := raw[:size]
		if kind == protowire.BytesType {
			var n int
			value, n = protowire.ConsumeBytes(value)
			if n < 0 {
				return ErrInvalid
			}
		}
		if err := visit(number, kind, value); err != nil {
			return err
		}
		raw = raw[size:]
	}
	return nil
}

func geoCountryHeader(raw []byte) (code string, reverse bool, err error) {
	err = geoFields(raw, func(number protowire.Number, kind protowire.Type, value []byte) error {
		switch number {
		case 1:
			if kind != protowire.BytesType || len(value) > 64 {
				return ErrInvalid
			}
			code = string(value)
		case 3:
			flag, n := protowire.ConsumeVarint(value)
			if kind != protowire.VarintType || n < 0 {
				return ErrInvalid
			}
			reverse = flag != 0
		}
		return nil
	})
	return
}

func geoCIDR(raw []byte) (netip.Prefix, error) {
	if len(raw) > 512 {
		return netip.Prefix{}, ErrInvalid
	}
	var address []byte
	var bits uint64
	err := geoFields(raw, func(number protowire.Number, kind protowire.Type, value []byte) error {
		switch number {
		case 1:
			if kind != protowire.BytesType || len(value) != 4 && len(value) != 16 {
				return ErrInvalid
			}
			address = value
		case 2:
			var n int
			bits, n = protowire.ConsumeVarint(value)
			if kind != protowire.VarintType || n < 0 {
				return ErrInvalid
			}
		}
		return nil
	})
	ip, ok := netip.AddrFromSlice(address)
	if err != nil || !ok || ip.Is4In6() || bits > uint64(ip.BitLen()) {
		return netip.Prefix{}, ErrInvalid
	}
	return netip.PrefixFrom(ip, int(bits)).Masked(), nil
}
