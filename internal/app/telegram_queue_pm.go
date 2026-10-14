package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (a *queuedTelegram) GetChat(ctx context.Context, id int64) (telegram.ChatInfo, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (telegram.ChatInfo, error) { return a.client.GetChat(callCtx, id) })
}
func (a *queuedTelegram) CreatePMTopic(ctx context.Context, name string, guard func(context.Context) (int64, error)) (telegram.ForumTopic, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (telegram.ForumTopic, error) {
		id, err := guard(callCtx)
		if err != nil {
			return telegram.ForumTopic{}, err
		}
		return a.client.CreateForumTopic(callCtx, id, name)
	})
}
func (a *queuedTelegram) CopyPMMessage(ctx context.Context, guard func(context.Context) (telegram.CopyMessageRequest, error)) (int64, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (int64, error) {
		input, err := guard(callCtx)
		if err != nil {
			return 0, err
		}
		return a.client.CopyMessage(callCtx, input)
	})
}

func (a *queuedTelegram) SendPMText(ctx context.Context, guard func(context.Context) (telegram.PMTextRequest, error)) (int64, error) {
	return upstreamqueue.Do(ctx, a.queue, func(ctx context.Context) (int64, error) {
		input, err := guard(ctx)
		if err != nil {
			return 0, err
		}
		return a.client.SendPMText(ctx, input)
	})
}
func (a *queuedTelegram) PublishPMProfile(ctx context.Context, guard func(context.Context) (telegram.TopicProfileRequest, error)) (int64, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (int64, error) {
		input, err := guard(callCtx)
		if err != nil {
			return 0, err
		}
		return a.client.PublishTopicProfile(callCtx, input)
	})
}
func (a *queuedTelegram) EditPMProfile(ctx context.Context, guard func(context.Context) (telegram.TopicProfileRequest, error)) (telegram.Message, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (telegram.Message, error) {
		input, err := guard(callCtx)
		if err != nil {
			return telegram.Message{}, err
		}
		return a.client.EditTopicProfile(callCtx, input)
	})
}
func (a *queuedTelegram) AnswerCallbackQuery(ctx context.Context, id, text string, alert bool) error {
	return upstreamqueue.Execute(ctx, a.queue, func(callCtx context.Context) error { return a.client.AnswerCallbackQuery(callCtx, id, text, alert) })
}
