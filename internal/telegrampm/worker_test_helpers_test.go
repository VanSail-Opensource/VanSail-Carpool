package telegrampm

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type workerRepo struct {
	WorkerRepository
	conversation   model.PMConversation
	operation      providerops.Operation
	items          map[string]providerops.Item
	qualified      bool
	resets, claims int
}

func (r *workerRepo) PMConversation(context.Context, string) (model.PMConversation, error) {
	return r.conversation, nil
}
func (r *workerRepo) PanelEntryQualified(context.Context, int64) (bool, error) {
	return r.qualified, nil
}
func (r *workerRepo) UserByID(context.Context, string) (model.User, error) {
	return model.User{ID: "admin", TelegramID: 99, Role: "admin"}, nil
}
func (r *workerRepo) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	out := []providerops.Item{}
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}
func (r *workerRepo) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	r.operation.Receipt.Status = "processing"
	return r.operation, nil
}
func (r *workerRepo) BeginProviderOperationItemAttempt(_ context.Context, _, key string, _ time.Time) (providerops.Item, error) {
	item := r.items[key]
	item.Status = providerops.StatusProcessing
	r.items[key] = item
	return item, nil
}
func (r *workerRepo) CompleteProviderOperationItem(_ context.Context, _, key string, c providerops.Completion, _ time.Time) (providerops.Item, error) {
	item := r.items[key]
	item.Status, item.ProviderReference, item.ErrorCode = c.Status, c.ProviderReference, c.ErrorCode
	r.items[key] = item
	return item, nil
}
func (r *workerRepo) CompleteProviderOperation(_ context.Context, _ string, c providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.operation.Receipt.Status = string(c.Status)
	r.operation.Receipt.ErrorCode = &c.ErrorCode
	return r.operation, nil
}
func (r *workerRepo) ClaimPMTopic(_ context.Context, _, operation string, _ time.Time) (bool, error) {
	if r.conversation.TopicState != "new" {
		return false, nil
	}
	r.claims++
	r.conversation.TopicState, r.conversation.TopicOperationID = "creating", operation
	return true, nil
}
func (r *workerRepo) SavePMTopic(_ context.Context, _, operation string, topic int64, _ time.Time) error {
	r.conversation.TopicState, r.conversation.TopicOperationID, r.conversation.TopicID = "ready", operation, topic
	return nil
}
func (r *workerRepo) ReleasePMTopic(_ context.Context, _, _ string, uncertain bool, _ time.Time) error {
	r.conversation.TopicState = "new"
	if uncertain {
		r.conversation.TopicState = "pending_review"
	}
	return nil
}
func (r *workerRepo) SavePMProfile(_ context.Context, _ string, _, message int64, _ time.Time) error {
	r.conversation.ProfileMessageID, r.conversation.ProfileState = message, "ready"
	return nil
}
func (r *workerRepo) ClearPMProfile(context.Context, string, int64) error {
	r.conversation.ProfileMessageID = 0
	return nil
}
func (r *workerRepo) PMProfileUncertain(context.Context, string, string) (bool, error) {
	return r.conversation.ProfileState == "pending_review", nil
}
func (r *workerRepo) MarkPMProfileUncertain(context.Context, string) error {
	r.conversation.ProfileState = "pending_review"
	return nil
}
func (r *workerRepo) RequeuePMItem(_ context.Context, _, key string, _ time.Time) error {
	item := r.items[key]
	item.Status = providerops.StatusQueued
	r.items[key] = item
	return nil
}
func (r *workerRepo) ResetMissingPMTopic(context.Context, string, string, int64, time.Time) error {
	r.resets++
	r.conversation.TopicID, r.conversation.ProfileMessageID = 0, 0
	r.conversation.TopicState = "new"
	for key, item := range r.items {
		item.Status = providerops.StatusQueued
		r.items[key] = item
	}
	return nil
}
func (r *workerRepo) SavePMTopicRepair(_ context.Context, _, operation string, topic, profile int64, _ time.Time) error {
	r.conversation.TopicID, r.conversation.ProfileMessageID = topic, profile
	r.conversation.TopicState, r.conversation.ProfileState, r.conversation.TopicOperationID = "ready", "ready", operation
	return nil
}

type workerSender struct {
	beforeCopy                       func()
	createErr, profileErr, copyErr   error
	creates, profiles, edits, copies int
	request                          telegram.CopyMessageRequest
	editResult                       telegram.Message
}

func (s *workerSender) CreatePMTopic(ctx context.Context, _ string, guard func(context.Context) (int64, error)) (telegram.ForumTopic, error) {
	if _, err := guard(ctx); err != nil {
		return telegram.ForumTopic{}, err
	}
	s.creates++
	return telegram.ForumTopic{MessageThreadID: 60}, s.createErr
}
func (s *workerSender) PublishPMProfile(ctx context.Context, guard func(context.Context) (telegram.TopicProfileRequest, error)) (int64, error) {
	if _, err := guard(ctx); err != nil {
		return 0, err
	}
	s.profiles++
	return 61, s.profileErr
}
func (s *workerSender) EditPMProfile(ctx context.Context, guard func(context.Context) (telegram.TopicProfileRequest, error)) (telegram.Message, error) {
	request, err := guard(ctx)
	if err != nil {
		return telegram.Message{}, err
	}
	s.edits++
	if s.editResult.MessageID > 0 {
		return s.editResult, s.profileErr
	}
	return telegram.Message{MessageID: request.MessageID, MessageThreadID: request.MessageThreadID, Chat: telegram.Chat{ID: request.ChatID}}, s.profileErr
}
func (s *workerSender) CopyPMMessage(ctx context.Context, guard func(context.Context) (telegram.CopyMessageRequest, error)) (int64, error) {
	if s.beforeCopy != nil {
		s.beforeCopy()
	}
	request, err := guard(ctx)
	if err != nil {
		return 0, err
	}
	s.copies++
	s.request = request
	return 62, s.copyErr
}
func (*workerSender) SendMarkdownV2Message(context.Context, int64, int64, string) error {
	return fmt.Errorf("unexpected notice")
}

func (s *workerSender) SendPMText(ctx context.Context, guard func(context.Context) (telegram.PMTextRequest, error)) (int64, error) {
	if _, err := guard(ctx); err != nil {
		return 0, err
	}
	return 63, s.copyErr
}

func newWorkerFixture() (*Worker, *workerRepo, *workerSender) {
	operation := providerops.Operation{Receipt: model.OperationReceipt{ID: "operation", Kind: providerops.KindTelegramPMRelay, Status: "queued"}, ActorUserID: "member", OwnerUserID: "member"}
	r := &workerRepo{operation: operation, qualified: true, conversation: model.PMConversation{ID: "conversation", UserID: "member", TelegramID: 42, ChatID: -100123, FirstName: "Member", TopicState: "new", ProfileState: "missing"}, items: map[string]providerops.Item{}}
	for _, item := range []providerops.Item{{Key: "topic", TargetType: "pm_conversation", TargetID: "conversation"}, {Key: "profile", TargetType: "pm_conversation", TargetID: "conversation"}, {Key: "relay", TargetType: "pm_inbound_message", TargetID: "42:4"}} {
		item.Status = providerops.StatusQueued
		r.items[item.Key] = item
	}
	s := &workerSender{}
	return &Worker{Repository: r, Settings: pmSettings{enabled: true, group: "-100123"}, Sender: s, AdminIDs: map[int64]bool{99: true}}, r, s
}
