package usdt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBoundedResponseRequiresOneCompleteDocument(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"valid", `{"value":7}`, true},
		{"at limit", `{"value":7}` + strings.Repeat(" ", maxRPCResponseBytes-11), true},
		{"oversized whitespace", `{"value":7}` + strings.Repeat(" ", maxRPCResponseBytes), false},
		{"oversized string", `{"ignored":"` + strings.Repeat("x", maxRPCResponseBytes) + `"}`, false},
		{"second document", `{"value":7}{}`, false},
		{"truncated", `{"value":`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result struct {
				Value int `json:"value"`
			}
			if err := decodeResponse(strings.NewReader(tc.body), &result); (err == nil) != tc.ok {
				t.Fatalf("decode=%v, want success=%v", err, tc.ok)
			}
		})
	}
}

func TestTypedRPCEnvelopeKeepsValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"typed result", `{"id":1,"result":[{"value":7}]}`, true},
		{"null", `{"id":1,"result":null}`, false},
		{"absent", `{"id":1}`, false},
		{"wrong id", `{"id":2,"result":[]}`, false},
		{"provider error", `{"id":1,"result":[],"error":{"code":-1}}`, false},
		{"wrong type", `{"id":1,"result":"bad"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := testClient(t, "ethereum", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			var result []struct {
				Value int `json:"value"`
			}
			if err := c.rpc(context.Background(), "eth_getLogs", []any{}, &result); (err == nil) != tc.ok {
				t.Fatalf("rpc=%v, want success=%v", err, tc.ok)
			}
			if tc.ok && (len(result) != 1 || result[0].Value != 7) {
				t.Fatal("typed destination was not decoded")
			}
		})
	}
}

func TestGraphQLEnvelopeRequiresDataWithoutErrors(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":{"value":7},"errors":[{}]}`} {
		c, _ := testClient(t, "aptos", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		var result struct {
			Value int `json:"value"`
		}
		if err := c.graphql(context.Background(), "query{}", nil, &result); err != ErrRPC {
			t.Fatalf("invalid envelope accepted: %s, error=%v", body, err)
		}
	}
}

func BenchmarkRPCResponseDecoding(b *testing.B) {
	body := `{"id":1,"result":{"value":"` + strings.Repeat("x", 1024*1024) + `"}}`
	for _, typed := range []bool{false, true} {
		name := "raw_envelope"
		if typed {
			name = "typed_envelope"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var result struct {
					Value string `json:"value"`
				}
				if typed {
					reply := typedRPCReply{Result: responseValue{target: &result}}
					if err := decodeResponse(strings.NewReader(body), &reply); err != nil {
						b.Fatal(err)
					}
				} else {
					data, err := io.ReadAll(strings.NewReader(body))
					if err != nil {
						b.Fatal(err)
					}
					var reply rpcReply
					if err := json.Unmarshal(data, &reply); err != nil {
						b.Fatal(err)
					}
					if err := decodeReply(reply, 1, &result); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
