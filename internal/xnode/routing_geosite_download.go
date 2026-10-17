package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type routingAssetResult struct {
	domains map[string][]string
	hash    string
}

func (s *Service) downloadRoutingGeoSite(ctx context.Context, config GlobalConfig, selectors []string) (routingAssetResult, error) {
	return upstreamqueue.Do(ctx, s.validationQueue, func(ctx context.Context) (routingAssetResult, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.GeoSiteURL, nil)
		if err != nil {
			return routingAssetResult{}, ErrInvalid
		}
		client := http.Client{Transport: publicTransport(), Timeout: time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) > 3 || !validHTTPS(r.URL.String()) {
				return ErrInvalid
			}
			return nil
		}}
		response, err := client.Do(req)
		if err != nil {
			return routingAssetResult{}, ErrUnavailable
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return routingAssetResult{}, ErrUnavailable
		}
		file, err := os.CreateTemp("", "xnode-master-geosite-*.dat")
		if err != nil {
			return routingAssetResult{}, err
		}
		defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
		hasher := sha256.New()
		size, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(response.Body, maxGeoIPAsset+1))
		if err != nil || size == 0 || size > maxGeoIPAsset {
			return routingAssetResult{}, ErrInvalid
		}
		hash := hex.EncodeToString(hasher.Sum(nil))
		if config.GeoSiteHash != "" && !strings.EqualFold(config.GeoSiteHash, hash) {
			return routingAssetResult{}, ErrInvalid
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return routingAssetResult{}, err
		}
		domains, err := readRoutingGeoSite(ctx, file, selectors)
		return routingAssetResult{domains, hash}, err
	})
}
