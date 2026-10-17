package xnode

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// The unpredictable seed is encrypted desired state and never enters a member
// projection. Rekeying the whole graph isolates mixed deployment generations.
func routingCredentials(doc RoutingDocument, identity Identity, r routingmatchRule, e Endpoint) (Credentials, wire.TransitCredential) {
	scope := fmt.Sprintf("%d\n%s\n%s\n%s\n%s\n%s", identity.ProxyUserID, identity.CurrentPeriodID, r.From, r.To, e.InboundTag, doc.Dependencies)
	derive := func(label string) []byte {
		h := hmac.New(sha256.New, []byte(doc.Seed))
		_, _ = h.Write([]byte(label + "\n" + scope))
		return h.Sum(nil)
	}
	u := derive("uuid")
	u[6] = u[6]&15 | 64
	u[8] = u[8]&63 | 128
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", u[:4], u[4:6], u[6:8], u[8:10], u[10:16])
	return Credentials{UUID: uuid, Trojan: hex.EncodeToString(derive("password")), Shadowsocks: hex.EncodeToString(derive("classic")), SS2022_128: base64.StdEncoding.EncodeToString(derive("ss128")[:16]), SS2022_256: base64.StdEncoding.EncodeToString(derive("ss256"))}, wire.TransitCredential{SourceNodeID: r.From, InboundTag: e.InboundTag, Generation: hex.EncodeToString(derive("generation"))}
}

// Only identifiers affect derivation; matcher content is covered by the saved
// dependency digest. This alias keeps no renderer or credential in graph models.
type routingmatchRule struct{ ID, From, To string }
