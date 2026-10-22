package xnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

const DIYOperationKind = "xnode_diy"

type DIYDocument struct {
	Unchanged     bool
	Billing       *DIYBillingQuote
	Result        *DIYSaveResult
	NativeChanged bool
	UserID        string
	Revision      int64
	Cipher        string
}
type DIYWrite struct {
	Quote            string                           `json:"quote,omitempty"`
	Whitelist        *WhitelistPolicy                 `json:"whitelist,omitempty"`
	SplitProfiles    *[]SplitProfile                  `json:"splitProfiles,omitempty"`
	SettingsRevision int64                            `json:"settingsRevision"`
	RoutingRevision  int64                            `json:"routingRevision"`
	Overrides        subscription.ConnectionOverrides `json:"overrides"`
	Choices          []RoutingChoice                  `json:"choices"`
}

func (input *DIYWrite) UnmarshalJSON(raw []byte) error {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return errors.New("invalid DIY document")
	}
	for _, key := range []string{"settingsRevision", "routingRevision", "overrides", "choices"} {
		if values[key] == nil || bytes.Equal(bytes.TrimSpace(values[key]), []byte("null")) {
			return errors.New("missing DIY section or revision")
		}
	}
	type plain DIYWrite
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*plain)(input))
}

type DIYView struct {
	SplitHosts       []DIYSplitHost                     `json:"splitHosts"`
	Billing          DIYBillingQuote                    `json:"billing"`
	Whitelist        WhitelistView                      `json:"whitelist"`
	SplitProfiles    []SplitProfile                     `json:"splitProfiles"`
	SettingsRevision int64                              `json:"settingsRevision"`
	Overrides        subscription.ConnectionOverrides   `json:"overrides"`
	Routing          RoutingView                        `json:"routing"`
	Fields           []subscription.TuningField         `json:"fields"`
	Compatibility    []subscription.TuningApplicability `json:"compatibility"`
}
type DIYPreview struct {
	Billing       DIYBillingQuote                    `json:"billing"`
	Valid         bool                               `json:"valid"`
	FieldErrors   map[string]string                  `json:"fieldErrors"`
	Paths         []routingmatch.Path                `json:"paths"`
	Compatibility []subscription.TuningApplicability `json:"compatibility"`
}
type DIYSaveResult struct {
	Legacy            bool                   `json:"-"`
	Changed           bool                   `json:"changed"`
	Billing           *DIYBillingQuote       `json:"billing,omitempty"`
	DeploymentChanged bool                   `json:"deploymentChanged"`
	SettingsRevision  int64                  `json:"settingsRevision"`
	RoutingChanged    bool                   `json:"routingChanged"`
	Receipt           model.OperationReceipt `json:"receipt"`
}
type diyRepository interface {
	XNodeDIY(context.Context, string) (DIYDocument, error)
	ReplayXNodeDIY(context.Context, string, string, string) (model.OperationReceipt, bool, bool, error)
	ReplayXNodeDIYResult(context.Context, string, string, string) (DIYSaveResult, bool, error)
	SaveXNodeDIY(context.Context, DIYDocument, int64, *RoutingPlan, providerops.CreateInput, time.Time) (model.OperationReceipt, error)
}
