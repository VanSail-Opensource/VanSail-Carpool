package xnode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

// ProviderZone exposes public DNS names only, never provider credentials.
type ProviderZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProviderZones struct {
	Supported bool           `json:"supported"`
	Items     []ProviderZone `json:"items"`
}

// ProviderZones discovers names with saved credentials. Results live only in
// this request and the administrator's editor; no discovery inventory is stored.
func (s *Service) ProviderZones(ctx context.Context, id string) (ProviderZones, error) {
	result := ProviderZones{Items: []ProviderZone{}}
	global, err := s.Global(ctx)
	if err != nil {
		return result, err
	}
	provider, found := global.Providers[id]
	if !found {
		return result, ErrNotFound
	}
	if provider.Type != "cloudflare" && provider.Type != "cf" {
		return result, nil
	}
	result.Supported = true
	token := ""
	for _, key := range []string{"CLOUDFLARE_DNS_API_TOKEN", "CF_API_TOKEN", "CLOUDFLARE_API_TOKEN"} {
		if token = strings.TrimSpace(provider.Credentials[key]); token != "" {
			break
		}
	}
	if token == "" {
		return result, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	result.Items, err = listCloudflareZones(ctx, s.validationQueue, client, token)
	return result, err
}

type zonePage struct {
	Success bool           `json:"success"`
	Result  []ProviderZone `json:"result"`
	Info    struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

// Each page enters the existing external queue separately. Bounds and the
// overall deadline also stop broken/repeating pagination without partial success.
func listCloudflareZones(ctx context.Context, queue *upstreamqueue.Queue, client *http.Client, token string) ([]ProviderZone, error) {
	items := []ProviderZone{}
	seen := map[string]bool{}
	for page := 1; page <= 1000; page++ {
		value, err := upstreamqueue.Do(ctx, queue, func(ctx context.Context) (zonePage, error) {
			return fetchCloudflareZones(ctx, client, token, page)
		})
		if err != nil {
			return nil, err
		}
		if value.Info.Page != 0 && value.Info.Page != page {
			return nil, ErrUnavailable
		}
		for _, zone := range value.Result {
			if zone.ID == "" || zone.Name == "" || len(zone.Name) > 253 || seen[zone.ID] {
				return nil, ErrUnavailable
			}
			seen[zone.ID] = true
			items = append(items, zone)
		}
		if value.Info.TotalPages > 0 {
			if page >= value.Info.TotalPages {
				return items, nil
			}
			if len(value.Result) == 0 {
				return nil, ErrUnavailable
			}
		} else if len(value.Result) < 50 {
			return items, nil
		}
	}
	return nil, ErrUnavailable
}

func fetchCloudflareZones(ctx context.Context, client *http.Client, token string, page int) (zonePage, error) {
	var value zonePage
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/zones?per_page=50&page="+strconv.Itoa(page), nil)
	if err != nil {
		return value, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return value, ctx.Err()
		}
		return value, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return value, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 || json.Unmarshal(raw, &value) != nil || !value.Success || value.Result == nil {
		return zonePage{}, ErrUnavailable
	}
	return value, nil
}
