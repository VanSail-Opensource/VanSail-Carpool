package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
	"strings"
	"time"
)

const questionnaireSelect = `SELECT q.id,q.title,q.description,q.form_url,q.reward_txb_minor,q.status,q.closes_at,
	(SELECT COUNT(*) FROM questionnaire_participants p WHERE p.questionnaire_id=q.id),
	(SELECT COUNT(*) FROM questionnaire_participants p WHERE p.questionnaire_id=q.id AND p.awarded_at IS NOT NULL),
	q.created_at,q.updated_at FROM questionnaires q`

// SaveQuestionnaire creates or updates a draft, active, or closed questionnaire.

func (s *Store) retryAttempt_SaveQuestionnaire(ctx context.Context, input questionnaires.QuestionnaireInput, now time.Time) (questionnaires.Questionnaire, error) {
	if err := input.Validate(); err != nil {
		return questionnaires.Questionnaire{}, err
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.FormURL = strings.TrimSpace(input.FormURL)
	now = now.UTC()
	if input.ClosesAt != nil {
		value := input.ClosesAt.UTC()
		input.ClosesAt = &value
		if input.Status == questionnaires.StatusActive && !value.After(now) {
			return questionnaires.Questionnaire{}, questionnaires.ErrInvalidInput
		}
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return questionnaires.Questionnaire{}, fmt.Errorf("begin questionnaire save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if input.Status == questionnaires.StatusActive {
		var activeID string
		var activeClosesAt sql.NullString
		loadErr := tx.QueryRowContext(ctx, `SELECT id,closes_at FROM questionnaires WHERE status='active'`).Scan(&activeID, &activeClosesAt)
		if loadErr != nil && !errors.Is(loadErr, sql.ErrNoRows) {
			return questionnaires.Questionnaire{}, fmt.Errorf("load active questionnaire: %w", loadErr)
		}
		if loadErr == nil && activeClosesAt.Valid {
			closes, parseErr := parseStamp(activeClosesAt.String)
			if parseErr != nil {
				return questionnaires.Questionnaire{}, parseErr
			}
			if !closes.After(now) {
				if _, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='closed',updated_at=? WHERE id=? AND status='active'`, stamp(now), activeID); err != nil {
					return questionnaires.Questionnaire{}, fmt.Errorf("close expired questionnaire: %w", err)
				}
			}
		}
	}
	if input.ID == "" {
		input.ID, err = ids.New()
		if err != nil {
			return questionnaires.Questionnaire{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO questionnaires(id,title,description,form_url,reward_txb_minor,status,closes_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
			input.ID, input.Title, input.Description, input.FormURL, input.RewardTXBMinor, input.Status, nullableStamp(input.ClosesAt), stamp(now), stamp(now))
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE questionnaires SET title=?,description=?,form_url=?,reward_txb_minor=?,status=?,closes_at=?,updated_at=?
			WHERE id=? AND status IN ('draft','active','closed')`, input.Title, input.Description, input.FormURL, input.RewardTXBMinor, input.Status, nullableStamp(input.ClosesAt), stamp(now), input.ID)
		if err == nil {
			if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
				return questionnaires.Questionnaire{}, rowsErr
			} else if affected == 0 {
				return questionnaires.Questionnaire{}, ErrConflict
			}
		}
	}
	if err != nil {
		if isUniqueConstraint(err) {
			return questionnaires.Questionnaire{}, ErrConflict
		}
		return questionnaires.Questionnaire{}, fmt.Errorf("save questionnaire: %w", err)
	}
	questionnaire, err := questionnaireByID(ctx, tx, input.ID)
	if err != nil {
		return questionnaires.Questionnaire{}, err
	}
	if err := tx.Commit(); err != nil {
		return questionnaires.Questionnaire{}, fmt.Errorf("commit questionnaire save: %w", err)
	}
	return questionnaire, nil
}

// ListQuestionnaires returns newest questionnaire history first.

func (s *Store) ListQuestionnaires(ctx context.Context) ([]questionnaires.Questionnaire, error) {
	rows, err := s.db.QueryContext(ctx, questionnaireSelect+` ORDER BY q.created_at DESC,q.id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]questionnaires.Questionnaire, 0)
	for rows.Next() {
		questionnaire, scanErr := scanQuestionnaire(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, questionnaire)
	}
	return result, rows.Err()
}

// ActiveQuestionnaire returns the single active questionnaire, if present.

func (s *Store) ActiveQuestionnaire(ctx context.Context, now time.Time) (*questionnaires.Questionnaire, error) {
	questionnaire, err := scanQuestionnaire(s.db.QueryRowContext(ctx, questionnaireSelect+` WHERE q.status='active'`))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if questionnaire.ClosesAt != nil && !questionnaire.ClosesAt.After(now.UTC()) {
		return nil, nil
	}
	return &questionnaire, nil
}

// EnsureQuestionnaireParticipant returns a durable code, generating it once.
