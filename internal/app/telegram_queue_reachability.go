package app

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (a *queuedTelegram) SendChatAction(ctx context.Context, id int64) (bool, error) {
	ctx = upstreamqueue.WithKey(ctx, "chat:"+fmt.Sprint(id))
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (bool, error) { return a.client.SendChatAction(callCtx, id) })
}
