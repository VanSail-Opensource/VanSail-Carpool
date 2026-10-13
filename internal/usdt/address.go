package usdt

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func decodeBase58(raw string) ([]byte, error) {
	if len(raw) < 20 || len(raw) > 50 {
		return nil, ErrConfiguration
	}
	n := new(big.Int)
	for _, ch := range raw {
		index := strings.IndexRune(base58Alphabet, ch)
		if index < 0 {
			return nil, ErrConfiguration
		}
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(index)))
	}
	zeros := 0
	for zeros < len(raw) && raw[zeros] == '1' {
		zeros++
	}
	return append(make([]byte, zeros), n.Bytes()...), nil
}

func encodeBase58(raw []byte) string {
	n, base, remainder := new(big.Int).SetBytes(raw), big.NewInt(58), new(big.Int)
	result := []byte{}
	for n.Sign() > 0 {
		n.QuoRem(n, base, remainder)
		result = append(result, base58Alphabet[remainder.Int64()])
	}
	for _, b := range raw {
		if b != 0 {
			break
		}
		result = append(result, '1')
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

// NormalizeAddress rejects invalid or testnet addresses and canonicalizes comparisons.
func NormalizeAddress(spec NetworkSpec, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch spec.Family {
	case "evm", "aptos":
		digits := strings.TrimPrefix(strings.ToLower(raw), "0x")
		size := 40
		if spec.Family == "aptos" {
			size = 64
			if len(digits) > 0 && len(digits) < size {
				digits = strings.Repeat("0", size-len(digits)) + digits
			}
		}
		if len(digits) != size {
			return "", ErrConfiguration
		}
		decoded, err := hex.DecodeString(digits)
		if err != nil || new(big.Int).SetBytes(decoded).Sign() == 0 {
			return "", ErrConfiguration
		}
		return "0x" + digits, nil
	case "tron":
		decoded, err := decodeBase58(raw)
		if err != nil || len(decoded) != 25 || decoded[0] != 0x41 {
			return "", ErrConfiguration
		}
		first := sha256.Sum256(decoded[:21])
		second := sha256.Sum256(first[:])
		if string(decoded[21:]) != string(second[:4]) {
			return "", ErrConfiguration
		}
		return raw, nil
	case "solana":
		decoded, err := decodeBase58(raw)
		if err != nil || len(decoded) != 32 {
			return "", ErrConfiguration
		}
		return raw, nil
	case "ton":
		if strings.HasPrefix(raw, "0:") && len(raw) == 66 {
			if _, err := hex.DecodeString(raw[2:]); err == nil {
				return strings.ToLower(raw), nil
			}
		}
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.ReplaceAll(strings.ReplaceAll(raw, "+", "-"), "/", "_"), "="))
		if err != nil || len(decoded) != 36 || (decoded[0] != 0x11 && decoded[0] != 0x51) || decoded[1] != 0 {
			return "", ErrConfiguration
		}
		crc := uint16(0)
		for _, b := range decoded[:34] {
			crc ^= uint16(b) << 8
			for range 8 {
				if crc&0x8000 != 0 {
					crc = (crc << 1) ^ 0x1021
				} else {
					crc <<= 1
				}
			}
		}
		if decoded[34] != byte(crc>>8) || decoded[35] != byte(crc) {
			return "", ErrConfiguration
		}
		return "0:" + hex.EncodeToString(decoded[2:34]), nil
	}
	return "", ErrConfiguration
}

func tronHexAddress(raw string) (string, error) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(raw, "0x"))
	if err != nil {
		return "", ErrProof
	}
	if len(decoded) == 20 {
		decoded = append([]byte{0x41}, decoded...)
	}
	if len(decoded) != 21 || decoded[0] != 0x41 {
		return "", ErrProof
	}
	first := sha256.Sum256(decoded)
	second := sha256.Sum256(first[:])
	return encodeBase58(append(decoded, second[:4]...)), nil
}
