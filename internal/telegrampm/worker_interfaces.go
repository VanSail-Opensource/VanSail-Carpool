package telegrampm

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"time"
)

type WorkerRepository interface {
	PMConversation(context.Context, string) (model.PMConversation, error)
	PanelEntryQualified(context.Context, int64) (bool, error)
	UserByID(context.Context, string) (model.User, error)
	ClaimPMTopic(context.Context, string, string, time.Time) (bool, error)
	SavePMTopic(context.Context, string, string, int64, time.Time) error
	ReleasePMTopic(context.Context, string, string, bool, time.Time) error
	SavePMProfile(context.Context, string, int64, int64, time.Time) error
	ClearPMProfile(context.Context, string, int64) error
	MarkPMProfileUncertain(context.Context, string) error
	PMProfileUncertain(context.Context, string, string) (bool, error)
	RequeuePMItem(context.Context, string, string, time.Time) error
	ResetMissingPMTopic(context.Context, string, string, int64, time.Time) error
	SavePMTopicRepair(context.Context, string, string, int64, int64, time.Time) error
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
}

// ProfileFactsReader builds a fresh, aggregate-only view for the topic profile card.
type ProfileFactsReader interface {
	PMProfileFacts(context.Context, string) (ProfileFacts, error)
}

type Sender interface {
	SendPMText(context.Context, func(context.Context) (telegram.PMTextRequest, error)) (int64, error)
	CreatePMTopic(context.Context, string, func(context.Context) (int64, error)) (telegram.ForumTopic, error)
	CopyPMMessage(context.Context, func(context.Context) (telegram.CopyMessageRequest, error)) (int64, error)
	PublishPMProfile(context.Context, func(context.Context) (telegram.TopicProfileRequest, error)) (int64, error)
	EditPMProfile(context.Context, func(context.Context) (telegram.TopicProfileRequest, error)) (telegram.Message, error)
	SendMarkdownV2Message(context.Context, int64, int64, string) error
}
