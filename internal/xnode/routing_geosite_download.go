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
		response, err := publicAssetClient(time.Minute).Do(req)
		if err != nil {
			return routingAssetResult{}, assetRequestFailure("GeoSite asset", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return routingAssetResult{}, assetResponseFailure("GeoSite asset", response.StatusCode)
		}
		file, err := os.CreateTemp("", "xnode-master-geosite-*.dat")
		if err != nil {
			return routingAssetResult{}, assetStorageFailure()
		}
		defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
		hasher := sha256.New()
		size, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(response.Body, maxGeoIPAsset+1))
		if err != nil {
			return routingAssetResult{}, assetTransferFailure("GeoSite asset", err)
		}
		if size == 0 || size > maxGeoIPAsset {
			return routingAssetResult{}, ErrInvalid
		}
		hash := hex.EncodeToString(hasher.Sum(nil))
		if config.GeoSiteHash != "" && !strings.EqualFold(config.GeoSiteHash, hash) {
			return routingAssetResult{}, ErrInvalid
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return routingAssetResult{}, assetStorageFailure()
		}
		domains, err := readRoutingGeoSite(ctx, file, selectors)
		if err != nil {
			return routingAssetResult{}, assetDecodeFailure(err)
		}
		return routingAssetResult{domains, hash}, err
	})
}
