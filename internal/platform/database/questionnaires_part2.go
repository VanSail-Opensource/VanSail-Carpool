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

func (s *Store) EnsureQuestionnaireParticipant(ctx context.Context, questionnaireID, userID string, generator questionnaires.CodeGenerator, now time.Time) (questionnaires.Participant, error) {
	if strings.TrimSpace(questionnaireID) == "" || strings.TrimSpace(userID) == "" || generator == nil {
		return questionnaires.Participant{}, questionnaires.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return questionnaires.Participant{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := participantByUserTx(ctx, tx, questionnaireID, userID); loadErr == nil {
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return questionnaires.Participant{}, loadErr
	}
	var status questionnaires.Status
	var closesAt sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT status,closes_at FROM questionnaires WHERE id=?`, questionnaireID).Scan(&status, &closesAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return questionnaires.Participant{}, ErrNotFound
		}
		return questionnaires.Participant{}, err
	}
	if status != questionnaires.StatusActive {
		return questionnaires.Participant{}, ErrConflict
	}
	now = now.UTC()
	if closesAt.Valid {
		closes, parseErr := parseStamp(closesAt.String)
		if parseErr != nil {
			return questionnaires.Participant{}, parseErr
		}
		if !closes.After(now) {
			return questionnaires.Participant{}, ErrConflict
		}
	}
	participantID, err := ids.New()
	if err != nil {
		return questionnaires.Participant{}, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, codeErr := generator.NewCode()
		if codeErr != nil {
			return questionnaires.Participant{}, codeErr
		}
		code = questionnaires.NormalizeValidationCode(code)
		if code == "" || len(code) > 128 {
			return questionnaires.Participant{}, questionnaires.ErrInvalidInput
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO questionnaire_participants(id,questionnaire_id,user_id,validation_code,created_at) VALUES(?,?,?,?,?)`,
			participantID, questionnaireID, userID, code, stamp(now))
		if err == nil {
			if err := tx.Commit(); err != nil {
				return questionnaires.Participant{}, err
			}
			return s.questionnaireParticipantByID(ctx, participantID)
		}
		if !isUniqueConstraint(err) {
			return questionnaires.Participant{}, fmt.Errorf("create questionnaire participant: %w", err)
		}
	}
	return questionnaires.Participant{}, fmt.Errorf("generate unique questionnaire code: %w", ErrConflict)
}

// ListQuestionnaireParticipations returns a member's durable questionnaire history.

func (s *Store) ListQuestionnaireParticipations(ctx context.Context, userID string, limit int) ([]questionnaires.ParticipationHistory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT
		q.id,q.title,q.description,q.form_url,q.reward_txb_minor,q.status,q.closes_at,q.created_at,q.updated_at,
		(SELECT COUNT(*) FROM questionnaire_participants count_participants WHERE count_participants.questionnaire_id=q.id),
		(SELECT COUNT(*) FROM questionnaire_participants count_rewards WHERE count_rewards.questionnaire_id=q.id AND count_rewards.awarded_at IS NOT NULL),
		p.id,p.questionnaire_id,p.user_id,p.validation_code,p.awarded_at,p.created_at
		FROM questionnaire_participants p JOIN questionnaires q ON q.id=p.questionnaire_id
		WHERE p.user_id=? ORDER BY p.created_at DESC,p.id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list questionnaire participation history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]questionnaires.ParticipationHistory, 0)
	for rows.Next() {
		var item questionnaires.ParticipationHistory
		var questionnaireCreated, questionnaireUpdated, participantCreated string
		var closesAt, awarded sql.NullString
		if err := rows.Scan(&item.Questionnaire.ID, &item.Questionnaire.Title, &item.Questionnaire.Description, &item.Questionnaire.FormURL,
			&item.Questionnaire.RewardTXBMinor, &item.Questionnaire.Status, &closesAt, &questionnaireCreated, &questionnaireUpdated,
			&item.Questionnaire.ParticipantCount, &item.Questionnaire.RewardedCount,
			&item.Participation.ID, &item.Participation.QuestionnaireID, &item.Participation.UserID, &item.Participation.ValidationCode,
			&awarded, &participantCreated); err != nil {
			return nil, err
		}
		if item.Questionnaire.CreatedAt, err = parseStamp(questionnaireCreated); err != nil {
			return nil, err
		}
		if item.Questionnaire.UpdatedAt, err = parseStamp(questionnaireUpdated); err != nil {
			return nil, err
		}
		if closesAt.Valid {
			value, parseErr := parseStamp(closesAt.String)
			if parseErr != nil {
				return nil, parseErr
			}
			item.Questionnaire.ClosesAt = &value
		}
		if item.Participation.CreatedAt, err = parseStamp(participantCreated); err != nil {
			return nil, err
		}
		if awarded.Valid {
			value, parseErr := parseStamp(awarded.String)
			if parseErr != nil {
				return nil, parseErr
			}
			item.Participation.AwardedAt = &value
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
