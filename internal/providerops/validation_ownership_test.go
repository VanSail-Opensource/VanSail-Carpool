package providerops

import (
	"reflect"
	"slices"
	"testing"
)

func TestNormalizeCreateOwnsItemsWithoutChangingCallerInput(t *testing.T) {
	t.Parallel()
	input := CreateInput{ActorUserID: " actor ", OwnerUserID: " owner ", Kind: " command ",
		IdempotencyKey: " key ", RequestFingerprint: " request-fingerprint ",
		Items: []ItemInput{{Key: " first ", TargetType: " user ", TargetID: " target "}}}
	before := input
	before.Items = slices.Clone(input.Items)
	normalized, err := NormalizeCreate(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, before) {
		t.Fatal("normalization mutated caller-owned command", input, before)
	}
	if normalized.ActorUserID != "actor" || normalized.OwnerUserID != "owner" ||
		normalized.Items[0] != (ItemInput{Key: "first", TargetType: "user", TargetID: "target"}) {
		t.Fatal("normalized command lost canonical values", normalized)
	}
	normalized.Items[0].Key = "changed-result"
	if !reflect.DeepEqual(input, before) {
		t.Fatal("normalized result aliases caller-owned items", input)
	}
	again, err := NormalizeCreate(input)
	if err != nil || again.Items[0].Key != "first" {
		t.Fatal("one normalization changed a later retry", again, err)
	}
}

func TestRejectedCommandNormalizationPreservesEveryInputItem(t *testing.T) {
	t.Parallel()
	for _, items := range [][]ItemInput{
		{{Key: " first ", TargetType: " user ", TargetID: " target "}, {Key: "invalid", TargetType: "user"}},
		{{Key: " same ", TargetType: " user ", TargetID: " first "}, {Key: "same", TargetType: "user", TargetID: "second"}},
	} {
		input := CreateInput{ActorUserID: "actor", Kind: "command", IdempotencyKey: "key",
			RequestFingerprint: "request-fingerprint", Items: items}
		before := slices.Clone(items)
		if _, err := NormalizeCreate(input); err == nil {
			t.Fatal("invalid command accepted", input)
		}
		if !reflect.DeepEqual(items, before) {
			t.Fatal("rejected normalization partially mutated caller items", items, before)
		}
	}
}
