package xnode

import (
	"context"
	"time"
)

// FirewallFilter is either an AND/OR group or one typed condition.
type FirewallFilter struct {
	Op       string           `json:"op,omitempty"`
	Children []FirewallFilter `json:"children,omitempty"`
	Field    string           `json:"field,omitempty"`
	Value    string           `json:"value,omitempty"`
	From     string           `json:"from,omitempty"`
	To       string           `json:"to,omitempty"`
}

type FirewallRecordSearch struct {
	Filter FirewallFilter `json:"filter"`
	Cursor string         `json:"cursor,omitempty"`
	Limit  int            `json:"limit"`
}

type FirewallRecord struct {
	ID           string     `json:"id"`
	IP           string     `json:"ip"`
	Prefix       string     `json:"prefix"`
	Rule         string     `json:"rule"`
	Count        string     `json:"count"`
	DetectedAt   time.Time  `json:"detectedAt"`
	Banned       bool       `json:"banned"`
	Generation   string     `json:"generation,omitempty"`
	Until        *time.Time `json:"until,omitempty"`
	WindowUntil  *time.Time `json:"windowUntil,omitempty"`
	PendingNodes *int       `json:"pendingNodes,omitempty"`
}

type FirewallRecordPage struct {
	Items      []FirewallRecord `json:"items"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

func (s *Service) SearchFirewallRecords(ctx context.Context, request FirewallRecordSearch) (FirewallRecordPage, error) {
	if request.Limit < 1 || request.Limit > 100 || len(request.Cursor) > 512 {
		return FirewallRecordPage{}, ErrInvalid
	}
	if _, err := CompileFirewallFilter(request.Filter); err != nil {
		return FirewallRecordPage{}, err
	}
	repo, ok := s.repo.(interface {
		SearchFirewallRecords(context.Context, FirewallRecordSearch, time.Time) (FirewallRecordPage, error)
	})
	if !ok {
		return FirewallRecordPage{}, ErrUnavailable
	}
	return repo.SearchFirewallRecords(ctx, request, s.now().UTC())
}
