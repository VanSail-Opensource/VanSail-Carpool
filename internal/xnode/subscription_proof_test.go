package xnode

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func firstSyncReadFixture() (wire.Config, *compileReads) {
	owner := Identity{ProxyUserID: 11, UserID: "owner", CredentialsCipher: "owner-material", CurrentPeriodID: "main-period"}
	child := Identity{ProxyUserID: 22, PAYGOwnerID: owner.UserID, CredentialsCipher: "child-material", CurrentPeriodID: "child-period"}
	service := Identity{ProxyUserID: 33, ServiceName: "standalone", CredentialsCipher: "service-material"}
	reads := &compileReads{
		byID:   map[int64]Identity{11: owner, 22: child, 33: service},
		byUser: map[string]Identity{owner.UserID: owner},
		users:  map[string]model.User{owner.UserID: {ID: owner.UserID, AccountGeneration: 7}},
	}
	config := wire.Config{Users: []wire.User{
		{ID: 11, AccountingPeriod: "main-period"},
		{ID: 22, AccountingPeriod: "child-period"},
		{ID: 33, AccountingPeriod: "service-period"},
	}}
	return config, reads
}

func TestFirstSyncCapturesTheExactCompilationSources(t *testing.T) {
	config, reads := firstSyncReadFixture()
	subjects, err := compiledFirstSyncSubjects(config, reads)
	if err != nil || len(subjects) != 2 {
		t.Fatal("member/standalone provenance mismatch", subjects, err)
	}
	want := FirstSyncSubject{
		ProxyUserID:         22,
		OwnerProxyUserID:    11,
		OwnerID:             "owner",
		AccountGeneration:   7,
		PeriodID:            "child-period",
		CredentialHash:      sha256.Sum256([]byte("child-material")),
		OwnerCredentialHash: sha256.Sum256([]byte("owner-material")),
	}
	if subjects[22] != want || subjects[11].CredentialHash != want.OwnerCredentialHash {
		t.Fatal("compiled native credentials were not bound to the captured owner")
	}
	encoded, err := json.Marshal(subjects[22])
	if err != nil || string(encoded) != "{}" {
		t.Fatal("private provenance entered a JSON payload", string(encoded), err)
	}
	// Reusing or replacing repository cache entries cannot relabel sent proof.
	reads.byID[22] = Identity{ProxyUserID: 22, PAYGOwnerID: "owner", CredentialsCipher: "new-child"}
	reads.byUser["owner"] = Identity{ProxyUserID: 11, UserID: "owner", CredentialsCipher: "new-owner"}
	reads.users["owner"] = model.User{ID: "owner", AccountGeneration: 8}
	if subjects[22] != want {
		t.Fatal("source mutation changed immutable applied membership")
	}
}

func TestFirstSyncCannotInventMissingCompilationProvenance(t *testing.T) {
	for _, missing := range []string{"principal", "owner", "account", "principal_material", "owner_material"} {
		t.Run(missing, func(t *testing.T) {
			config, reads := firstSyncReadFixture()
			switch missing {
			case "principal":
				delete(reads.byID, 22)
			case "owner":
				delete(reads.byUser, "owner")
			case "account":
				delete(reads.users, "owner")
			case "principal_material":
				value := reads.byID[22]
				value.CredentialsCipher = ""
				reads.byID[22] = value
			case "owner_material":
				value := reads.byUser["owner"]
				value.CredentialsCipher = ""
				reads.byUser["owner"] = value
			}
			if _, err := compiledFirstSyncSubjects(config, reads); !errors.Is(err, ErrConflict) {
				t.Fatal("missing source material was accepted", err)
			}
		})
	}
}
