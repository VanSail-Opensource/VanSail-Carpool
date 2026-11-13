package xnode

import (
	"context"
	"crypto/sha256"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Service) Enrollment(ctx context.Context, id string) (string, string, error) {
	return s.enrollment(ctx, id, "")
}

func (s *Service) enrollment(ctx context.Context, id, actor string) (string, string, error) {
	node, err := s.repo.XNodeNode(ctx, id)
	if err != nil {
		return "", "", err
	}
	if node.RetiredAt != nil {
		return "", "", ErrConflict
	}
	token, err := ids.Token(32)
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(token))
	if s.master != nil {
		err = s.master.rotateEnrollmentAs(ctx, id, hash[:], actor)
	} else {
		err = s.persistEnrollment(ctx, actor, id, hash[:])
	}
	if err != nil {
		return "", "", err
	}
	u := *s.publicURL
	u.Scheme = "wss"
	u.Path = "/v1/agents/connect"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), token, nil
}
