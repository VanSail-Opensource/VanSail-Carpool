package accounts

import (
	"context"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type cleanupTestTrace struct {
	mu    sync.Mutex
	calls []string
}

func (s *cleanupTestTrace) add(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, value)
}
func (s *cleanupTestTrace) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

type cleanupTestRepository struct {
	mu           sync.Mutex
	trace        *cleanupTestTrace
	probe        *model.AccountCleanupProbe
	observations []model.AccountCleanupObservation
	botIDs       []int64
	limit        int
}

func (r *cleanupTestRepository) SetAccountCleanupBot(ctx context.Context, id int64, at time.Time) (model.AccountCleanupHealth, error) {
	r.trace.add("bot-state")
	r.mu.Lock()
	defer r.mu.Unlock()
	r.botIDs = append(r.botIDs, id)
	return model.AccountCleanupHealth{BotID: id, Epoch: 1, Verified: id > 0, VerifiedAt: &at}, ctx.Err()
}
func (r *cleanupTestRepository) EnqueueAccountCleanupChecks(ctx context.Context, _ time.Time, limit int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limit = limit
	return ctx.Err()
}
func (r *cleanupTestRepository) AccountCleanupProbe(ctx context.Context, _ string, _ int64, _ string) (*model.AccountCleanupProbe, error) {
	r.trace.add("load-nonce")
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.probe == nil {
		return nil, ctx.Err()
	}
	p := *r.probe
	return &p, ctx.Err()
}
func (r *cleanupTestRepository) CompleteAccountCleanupProbe(ctx context.Context, _ model.AccountCleanupProbe, o model.AccountCleanupObservation, _ time.Time) (model.AccountCleanupCompletion, error) {
	r.trace.add("complete")
	if err := ctx.Err(); err != nil {
		return model.AccountCleanupCompletion{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observations = append(r.observations, o)
	return model.AccountCleanupCompletion{Accepted: true}, nil
}

type cleanupTestTelegram struct {
	trace                     *cleanupTestTrace
	action                    bool
	actionErr, chatErr, meErr error
	chat                      telegram.ChatInfo
	bot                       telegram.User
	actionHook                func(context.Context) (bool, error)
}

func (c *cleanupTestTelegram) SendChatAction(ctx context.Context, _ int64) (bool, error) {
	c.trace.add("action")
	if c.actionHook != nil {
		return c.actionHook(ctx)
	}
	return c.action, c.actionErr
}
func (c *cleanupTestTelegram) GetChat(context.Context, int64) (telegram.ChatInfo, error) {
	c.trace.add("chat")
	return c.chat, c.chatErr
}
func (c *cleanupTestTelegram) GetMe(context.Context) (telegram.User, error) {
	c.trace.add("getMe")
	return c.bot, c.meErr
}

func cleanupFixture() (*AccountCleanupService, *cleanupTestRepository, *cleanupTestTelegram) {
	trace := &cleanupTestTrace{}
	repo := &cleanupTestRepository{trace: trace, probe: &model.AccountCleanupProbe{UserID: "ordinary", TelegramID: 42, AccountGeneration: 1, BotID: 123, Epoch: 1, Nonce: "pending-nonce", StartedAt: time.Now().UTC()}}
	client := &cleanupTestTelegram{trace: trace, action: true, chat: telegram.ChatInfo{ID: 42, Type: "private"}, bot: telegram.User{ID: 123, IsBot: true, Username: "fixture"}}
	s, _ := NewAccountCleanup(repo, client, 123, nil)
	return s, repo, client
}

func cleanupFixtureJob() model.OutboxJob {
	return model.OutboxJob{Kind: AccountCleanupJob, Payload: `{"userId":"ordinary","generation":1,"nonce":"pending-nonce"}`}
}
