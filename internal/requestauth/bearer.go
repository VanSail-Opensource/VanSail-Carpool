package requestauth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// CheckBearer applies the existing body and per-principal request budgets to
// an authenticated API token. Bearer clients do not require browser signatures.
func (v *Verifier) CheckBearer(request *http.Request, id string, streaming bool) error {
	if v == nil || id == "" {
		return ErrRequired
	}
	if !streaming {
		if _, err := readAndRestore(request, v.maxBody); err != nil {
			return err
		}
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	now := v.now()
	return v.replays.add("api:"+id, hex.EncodeToString(nonce[:]), now, now.Add(v.window))
}
