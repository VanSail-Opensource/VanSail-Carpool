package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (w *luckyDrawOutbox) resultNotice(ctx context.Context, id, kind string) error {
	result, err := w.store.LuckyDrawResultByID(ctx, id)
	if err != nil {
		return err
	}
	user, err := w.store.UserByID(ctx, result.UserID)
	if err != nil {
		return err
	}
	key := kind + ":" + id
	if _, done, readErr := w.store.DrawDelivery(ctx, key); readErr != nil || done {
		return readErr
	}
	var chatID int64
	var body string
	if kind == "draw_instant_announcement" {
		chatID, err = w.groupID(ctx)
		if err != nil {
			return err
		}
		body = drawInstantMessage(user, result)
	} else {
		chatID = user.TelegramID
		body = drawPrivateMessage(result)
	}
	messageID, err := w.telegram.PublishMarkdownV2Message(ctx, chatID, body)
	if err != nil {
		return err
	}
	return w.store.RecordDrawDelivery(ctx, key, chatID, messageID, time.Now().UTC())
}
func (w *luckyDrawOutbox) complete(ctx context.Context, id string) error {
	draw, err := w.store.LuckyDrawByID(ctx, id)
	if err != nil {
		return err
	}
	if draw.Status != "completed" {
		return nil
	}
	results, err := w.store.RaffleResults(ctx, id)
	if err != nil {
		return err
	}
	users := make(map[string]model.User)
	for _, result := range results {
		if _, exists := users[result.UserID]; exists {
			continue
		}
		user, loadErr := w.store.UserByID(ctx, result.UserID)
		if loadErr != nil {
			return loadErr
		}
		users[result.UserID] = user
	}
	messages := drawRaffleResultMessages(draw, results, users)
	for index, body := range messages {
		key := "raffle:result:" + id + ":" + strconv.Itoa(index)
		if _, done, readErr := w.store.DrawDelivery(ctx, key); readErr != nil {
			return readErr
		} else if done {
			continue
		}
		messageID, sendErr := w.telegram.PublishMarkdownV2Message(ctx, draw.GroupChatID, body)
		if sendErr != nil {
			return fmt.Errorf("post raffle result part %d: %w", index+1, sendErr)
		}
		if err = w.store.RecordDrawDelivery(ctx, key, draw.GroupChatID, messageID, time.Now().UTC()); err != nil {
			return err
		}
	}
	return w.delete(ctx, id)
}
