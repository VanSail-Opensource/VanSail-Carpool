package xnode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (s *Service) SaveGlobal(ctx context.Context, actor string, next GlobalConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.Global(ctx)
	if err != nil {
		return err
	}
	for name, provider := range next.Providers {
		old := previous.Providers[name]
		if provider.Credentials == nil {
			provider.Credentials = old.Credentials
		} else {
			for key, value := range provider.Credentials {
				if value == "" {
					provider.Credentials[key] = old.Credentials[key]
				}
			}
		}
		next.Providers[name] = provider
	}
	if err = ValidateGlobal(next); err != nil {
		return err
	}
	if err = s.validateAffected(ctx, nil, nil, &next); err != nil {
		return err
	}
	for _, target := range []string{next.GeoIPURL, next.GeoSiteURL} {
		if target == "" {
			continue
		}
		if err = s.validateAssetLink(ctx, target); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err = s.settings.PutXNodeGlobal(ctx, actor, string(raw)); err != nil {
		return err
	}
	s.localCompileRevision.Add(1)
	return nil
}
func (s *Service) validateAssetLink(ctx context.Context, target string) error {
	return upstreamqueue.Execute(ctx, s.validationQueue, func(ctx context.Context) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return ErrInvalid
		}
		request.Header.Set("Range", "bytes=0-4095")
		client := &http.Client{Transport: publicTransport(), Timeout: 15 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > 3 || !validHTTPS(request.URL.String()) {
				return ErrInvalid
			}
			return nil
		}}
		response, err := client.Do(request)
		if err != nil {
			return ErrUnavailable
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
			return ErrUnavailable
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 4096))
		if err != nil || len(data) == 0 {
			return ErrUnavailable
		}
		return nil
	})
}
