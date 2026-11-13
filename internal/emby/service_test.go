package emby

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestHardenPolicyPreservesRemoteAccessAndForcesRestrictions(t *testing.T) {
	t.Parallel()
	current := Policy{
		"EnableRemoteAccess": json.RawMessage(`true`),
		"FutureField":        json.RawMessage(`{"nested":"kept"}`),
		"IsHidden":           json.RawMessage(`false`),
	}
	rating := int32(13)
	hardened := HardenPolicy(current, Preferences{MaxParentalRating: &rating, DisabledLibraryIDs: []string{"movies"}})
	checks := map[string]string{
		"EnableRemoteAccess":              "true",
		"FutureField":                     `{"nested":"kept"}`,
		"IsHidden":                        "true",
		"IsHiddenRemotely":                "true",
		"EnableRemoteControlOfOtherUsers": "false",
		"EnableSharedDeviceControl":       "false",
		"EnableAudioPlaybackTranscoding":  "false",
		"EnableVideoPlaybackTranscoding":  "false",
		"EnablePlaybackRemuxing":          "false",
		"EnableSyncTranscoding":           "false",
		"EnableMediaConversion":           "false",
		"EnableContentDownloading":        "false",
		"EnableSubtitleDownloading":       "false",
		"EnableAllFolders":                "true",
		"MaxParentalRating":               "13",
		"EnabledFolders":                  `[]`,
		"BlockedMediaFolders":             `["movies"]`,
	}
	for key, want := range checks {
		if got := string(hardened[key]); got != want {
			t.Errorf("policy[%s] = %s, want %s", key, got, want)
		}
	}
	if string(current["IsHidden"]) != "false" {
		t.Fatal("HardenPolicy mutated the fetched policy")
	}
}

func TestSetupPreparationUsesServerPriceAndProvisioningContext(t *testing.T) {
	t.Parallel()
	repository := &serviceRepository{baseUsername: "ada"}
	remote := newServiceRemote()
	secrets := &serviceSecrets{}
	service := NewService(repository, remote, fixedPrice(275), secrets)
	rating := int32(13)
	input, err := service.prepareSetup(context.Background(), "user-1", "private password", Preferences{MaxParentalRating: &rating, DisabledLibraryIDs: []string{"movies", "movies"}})
	if err != nil || input.SetupPriceTXBMinor != 275 || input.ID == "" || input.PasswordCiphertext == "" {
		t.Fatalf("prepareSetup() = (%+v, %v)", input, err)
	}
	if secrets.context != "emby.provisioning.password:user-1" || secrets.plaintext != "private password" {
		t.Fatalf("sealed secret = context %q plaintext %q", secrets.context, secrets.plaintext)
	}
	if input.BaseUsername != "ada" || input.SetupPriceTXBMinor != 275 || !reflect.DeepEqual(input.Preferences.DisabledLibraryIDs, []string{"movies"}) {
		t.Fatalf("prepared input = %+v", input)
	}
}

func TestProvisionCollisionHardeningAndAmbiguousReconciliation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		record            ProvisioningRecord
		seedBaseCollision bool
		seedCandidate     bool
		wantCreate        bool
	}{
		{
			name: "base collision receives stable suffix",
			record: ProvisioningRecord{Account: Account{ID: "account-1", UserID: "user-1", BaseUsername: "ada", Status: StatusQueued,
				Preferences: Preferences{DisabledLibraryIDs: []string{"movies"}}}, PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user-1"},
			seedBaseCollision: true, wantCreate: true,
		},
		{
			name: "ambiguous create is reconciled by persisted exact name",
			record: ProvisioningRecord{Account: Account{ID: "account-2", UserID: "user-2", BaseUsername: "river", CandidateUsername: "river", Status: StatusQueued,
				Preferences: Preferences{DisabledLibraryIDs: []string{"movies"}}}, PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user-2", CreateAttempted: true},
			seedCandidate: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &serviceRepository{record: test.record}
			remote := newServiceRemote()
			if test.seedBaseCollision {
				remote.addUser(RemoteUser{ID: "existing", Name: test.record.BaseUsername, Policy: Policy{}})
			}
			if test.seedCandidate {
				remote.addUser(RemoteUser{ID: "reconciled", Name: test.record.CandidateUsername, Policy: basePolicy()})
			}
			service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
			if err := service.HandleProvisionJob(context.Background(), test.record.ID); err != nil {
				t.Fatalf("Provision() error = %v", err)
			}
			if repository.record.Status != StatusActive || repository.record.PasswordCiphertext != "" || repository.record.RemoteUserID == "" {
				t.Fatalf("provisioned record = %+v", repository.record)
			}
			if test.wantCreate != (remote.createCalls == 1) {
				t.Fatalf("create calls = %d, wantCreate=%v", remote.createCalls, test.wantCreate)
			}
			if test.seedBaseCollision && repository.record.CandidateUsername != SuffixedUsername("ada", "user-1") {
				t.Fatalf("candidate = %q", repository.record.CandidateUsername)
			}
			if string(remote.updatedPolicy["EnableRemoteAccess"]) != "true" || string(remote.updatedPolicy["EnableAllFolders"]) != "true" ||
				string(remote.updatedPolicy["FutureField"]) != `"kept"` {
				t.Fatalf("updated policy = %#v", remote.updatedPolicy)
			}
			if string(remote.password) != "temporary" {
				t.Fatalf("password = %q", remote.password)
			}
		})
	}
}

func TestLinkedPreferencesAndPasswordNeverPersistPlaintext(t *testing.T) {
	t.Parallel()
	repository := &serviceRepository{record: ProvisioningRecord{Account: Account{
		ID: "account", UserID: "user", BaseUsername: "ada", RemoteUserID: "remote", RemoteUsername: "ada", Status: StatusActive,
	}}}
	remote := newServiceRemote()
	remote.addUser(RemoteUser{ID: "remote", Name: "ada", Policy: basePolicy()})
	service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
	rating := int32(13)
	updated, err := service.UpdatePreferences(context.Background(), "user", Preferences{MaxParentalRating: &rating, DisabledLibraryIDs: []string{"movies"}})
	if err != nil || updated.Preferences.MaxParentalRating == nil || *updated.Preferences.MaxParentalRating != 13 {
		t.Fatalf("UpdatePreferences() = (%+v, %v)", updated, err)
	}
	if string(remote.updatedPolicy["EnableRemoteAccess"]) != "true" || string(remote.updatedPolicy["EnableContentDownloading"]) != "false" {
		t.Fatalf("restricted policy = %#v", remote.updatedPolicy)
	}
	if err := service.ChangePassword(context.Background(), "user", "current", "replacement"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if string(remote.currentPassword) != "current" || string(remote.password) != "replacement" || repository.touchCalls != 1 {
		t.Fatalf("password call current=%q next=%q touches=%d", remote.currentPassword, remote.password, repository.touchCalls)
	}
	account, err := service.Account(context.Background(), "user")
	if err != nil || account.ID != "account" {
		t.Fatalf("Account() = (%+v, %v)", account, err)
	}
	accounts, err := service.ListAccounts(context.Background(), 10)
	if err != nil || len(accounts) != 1 || accounts[0].ID != "account" {
		t.Fatalf("ListAccounts() = (%+v, %v)", accounts, err)
	}
}

func TestVaultAdapterAndPriceFunc(t *testing.T) {
	t.Parallel()
	cipher := &serviceCipher{}
	box := NewSecretBox(cipher)
	sealed, err := box.Seal("emby.provisioning.password:user", []byte("secret"))
	if err != nil || sealed != "encrypted" || cipher.encryptContext != "emby.provisioning.password:user" {
		t.Fatalf("Seal() = (%q, %v), cipher=%+v", sealed, err, cipher)
	}
	opened, err := box.Open("emby.provisioning.password:user", sealed)
	if err != nil || string(opened) != "secret" {
		t.Fatalf("Open() = (%q, %v)", opened, err)
	}
	price, err := PriceFunc(func(context.Context) (int64, error) { return 250, nil }).EmbySetupPriceTXBMinor(context.Background())
	if err != nil || price != 250 {
		t.Fatalf("PriceFunc() = (%d, %v)", price, err)
	}
}
