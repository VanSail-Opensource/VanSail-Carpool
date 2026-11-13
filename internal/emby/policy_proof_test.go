package emby

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestPolicyRestrictionRequiresExplicitBooleanEvidence(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"EnableRemoteControlOfOtherUsers", "EnableSharedDeviceControl",
		"EnableAudioPlaybackTranscoding", "EnableVideoPlaybackTranscoding",
		"EnablePlaybackRemuxing", "EnableSyncTranscoding", "EnableMediaConversion",
		"EnableContentDownloading", "EnableSubtitleDownloading",
	} {
		t.Run(key, func(t *testing.T) {
			policy := HardenPolicy(Policy{}, Preferences{})
			if !PolicyMatchesPreferences(policy, Preferences{}) {
				t.Fatal("explicit hardened policy did not verify")
			}
			policy[key] = json.RawMessage("null")
			if PolicyMatchesPreferences(policy, Preferences{}) {
				t.Fatal("unknown restriction was accepted as proven false")
			}
		})
	}
}

type nullRestrictionRemote struct{ *serviceRemote }

func (r *nullRestrictionRemote) UpdatePolicy(ctx context.Context, id string, policy Policy) error {
	if err := r.serviceRemote.UpdatePolicy(ctx, id, policy); err != nil {
		return err
	}
	user := r.byID[id]
	user.Policy["EnableContentDownloading"] = json.RawMessage("null")
	r.addUser(user)
	return nil
}

func TestProvisionDoesNotActivateAnUnverifiedRestriction(t *testing.T) {
	t.Parallel()
	repository := &serviceRepository{record: ProvisioningRecord{
		Account:            Account{ID: "account", UserID: "user", BaseUsername: "ada", Status: StatusQueued},
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user",
	}}
	remote := &nullRestrictionRemote{serviceRemote: newServiceRemote()}
	service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
	err := service.Provision(context.Background(), "account")
	if !errors.Is(err, ErrPendingReview) || repository.record.Status != StatusPendingReview {
		t.Fatalf("unknown policy was accepted: status=%s error=%v", repository.record.Status, err)
	}
	if repository.refundCalls != 0 || repository.record.PasswordCiphertext == "" {
		t.Fatal("ambiguous policy lost its recoverable setup evidence")
	}
}
