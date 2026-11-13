package emby

import (
	"encoding/json"
	"slices"
	"sort"
)

type Cipher interface {
	Encrypt(context, plaintext string) (string, error)
	Decrypt(context, ciphertext string) (string, error)
}

type cipherSecretBox struct{ cipher Cipher }

// NewSecretBox adapts the process AES-GCM vault to the provisioning SecretBox.

func NewSecretBox(cipher Cipher) SecretBox { return cipherSecretBox{cipher: cipher} }

func (b cipherSecretBox) Seal(context string, plaintext []byte) (string, error) {
	return b.cipher.Encrypt(context, string(plaintext))
}

func (b cipherSecretBox) Open(context, ciphertext string) ([]byte, error) {
	plaintext, err := b.cipher.Decrypt(context, ciphertext)
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}

// HardenPolicy preserves the fetched complete policy, overlays the selected
// rating and folders, and reasserts the non-negotiable account restrictions.
// EnableRemoteAccess is deliberately left untouched.

func HardenPolicy(current Policy, preferences Preferences) Policy {
	policy := current.Clone()
	setPolicy(policy, "MaxParentalRating", preferences.MaxParentalRating)
	disabled := append([]string(nil), preferences.DisabledLibraryIDs...)
	if disabled == nil {
		disabled = []string{}
	}
	setPolicy(policy, "EnableAllFolders", true)
	setPolicy(policy, "EnabledFolders", []string{})
	setPolicy(policy, "BlockedMediaFolders", disabled)
	setPolicy(policy, "IsHidden", true)
	setPolicy(policy, "IsHiddenRemotely", true)
	setPolicy(policy, "EnableRemoteControlOfOtherUsers", false)
	setPolicy(policy, "EnableSharedDeviceControl", false)
	setPolicy(policy, "EnableAudioPlaybackTranscoding", false)
	setPolicy(policy, "EnableVideoPlaybackTranscoding", false)
	setPolicy(policy, "EnablePlaybackRemuxing", false)
	setPolicy(policy, "EnableSyncTranscoding", false)
	setPolicy(policy, "EnableMediaConversion", false)
	setPolicy(policy, "EnableContentDownloading", false)
	setPolicy(policy, "EnableSubtitleDownloading", false)
	return policy
}

// PolicyMatchesPreferences verifies the documented fields after an Emby
// policy write. The complete policy remains remote-owned; only these fields are
// compared before local disabled-folder IDs may be committed.

func PolicyMatchesPreferences(policy Policy, preferences Preferences) bool {
	var enableAll bool
	var enabled, blocked []string
	if json.Unmarshal(policy["EnableAllFolders"], &enableAll) != nil || !enableAll ||
		json.Unmarshal(policy["EnabledFolders"], &enabled) != nil || len(enabled) != 0 ||
		json.Unmarshal(policy["BlockedMediaFolders"], &blocked) != nil {
		return false
	}
	sort.Strings(blocked)
	expected := append([]string(nil), preferences.DisabledLibraryIDs...)
	sort.Strings(expected)
	if !slices.Equal(blocked, expected) {
		return false
	}
	for _, key := range []string{"IsHidden", "IsHiddenRemotely"} {
		if !policyBooleanEquals(policy, key, true) {
			return false
		}
	}
	for _, key := range []string{"EnableRemoteControlOfOtherUsers", "EnableSharedDeviceControl", "EnableAudioPlaybackTranscoding",
		"EnableVideoPlaybackTranscoding", "EnablePlaybackRemuxing", "EnableSyncTranscoding", "EnableMediaConversion",
		"EnableContentDownloading", "EnableSubtitleDownloading"} {
		if !policyBooleanEquals(policy, key, false) {
			return false
		}
	}
	var storedRating *int32
	if json.Unmarshal(policy["MaxParentalRating"], &storedRating) != nil {
		return false
	}
	return (storedRating == nil && preferences.MaxParentalRating == nil) ||
		(storedRating != nil && preferences.MaxParentalRating != nil && *storedRating == *preferences.MaxParentalRating)
}

func policyBooleanEquals(policy Policy, key string, expected bool) bool {
	var actual *bool
	return json.Unmarshal(policy[key], &actual) == nil && actual != nil && *actual == expected
}

func setPolicy(policy Policy, key string, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		// Every caller supplies JSON primitive values or string slices, for which
		// encoding/json cannot fail.
		return
	}
	policy[key] = encoded
}
