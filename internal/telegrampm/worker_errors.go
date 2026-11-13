package telegrampm

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func topicMissing(err error) bool {
	var api *telegram.APIError
	if !errors.As(err, &api) || api == nil || api.ErrorCode != 400 || !definiteRejection(err) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(api.Description)) {
	case "bad request: message thread not found", "bad request: topic not found", "bad request: topic deleted":
		return true
	default:
		return false
	}
}

func profileMissing(err error) bool {
	var api *telegram.APIError
	return errors.As(err, &api) && api != nil && api.ErrorCode == 400 && definiteRejection(err) &&
		strings.ToLower(strings.TrimSpace(api.Description)) == "bad request: message to edit not found"
}

func definiteRejection(err error) bool {
	var api *telegram.APIError
	if !errors.As(err, &api) || api == nil || api.HTTPStatus != api.ErrorCode {
		return false
	}
	switch api.ErrorCode {
	case 400, 401, 403:
		return api.RetryAfter == 0
	case 429:
		return api.RetryAfter >= 0
	default:
		return false
	}
}

func (w *Worker) callResult(ctx context.Context, run execution, item providerops.Item, err error, started, appendOnly bool, reference, uncertainCode string) (phaseResult, error) {
	if err == nil {
		return phaseResult{status: providerops.StatusSucceeded, reference: reference}, nil
	}
	if errors.Is(err, ErrTopicUncertain) && item.Key == "relay" && !started {
		persist, cancel := completionContext(ctx)
		defer cancel()
		if saveErr := w.Repository.RequeuePMItem(persist, run.operation.Receipt.ID, item.Key, time.Now().UTC()); saveErr != nil {
			return phaseResult{}, saveErr
		}
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_TOPIC_UNCERTAIN", deferred: true}, nil
	}
	for _, guard := range []struct {
		err  error
		code string
	}{{ErrDisabled, "PM_DISABLED"}, {ErrBlocked, "PM_BLOCKED"}, {ErrUnqualified, "PM_ENTRY_REQUIRED"}, {ErrTopicUncertain, "PM_TOPIC_UNCERTAIN"}, {model.ErrPMContentExpired, "PM_CONTENT_EXPIRED"}, {model.ErrPMContentInvalid, "PM_CONTENT_INVALID"}} {
		if errors.Is(err, guard.err) {
			return phaseResult{status: providerops.StatusFailed, code: guard.code}, nil
		}
	}
	var api *telegram.APIError
	rateLimited := errors.As(err, &api) && api != nil && api.ErrorCode == 429 && definiteRejection(err)
	if (!started || rateLimited || !appendOnly && !definiteRejection(err)) && run.job.Attempts < 10 {
		persist, cancel := completionContext(ctx)
		defer cancel()
		if saveErr := w.Repository.RequeuePMItem(persist, run.operation.Receipt.ID, item.Key, time.Now().UTC()); saveErr != nil {
			return phaseResult{}, saveErr
		}
		return phaseResult{}, err
	}
	if definiteRejection(err) || !started || !appendOnly {
		return phaseResult{status: providerops.StatusFailed, code: "PM_TELEGRAM_REJECTED"}, nil
	}
	return phaseResult{status: providerops.StatusPendingReview, code: uncertainCode}, nil
}
