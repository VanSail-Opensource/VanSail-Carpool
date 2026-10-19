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

const maxGeoIPAsset = 128 << 20

// resolveGeoIP streams the public asset through one existing queue slot and a
// temporary file; neither the full download nor a full protobuf tree is retained.
func (s *Service) resolveGeoIP(ctx context.Context, config GlobalConfig) ([]string, string, error) {
	type result struct {
		prefixes   []string
		generation string
	}
	value, err := upstreamqueue.Do(ctx, s.validationQueue, func(ctx context.Context) (result, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.GeoIPURL, nil)
		if err != nil {
			return result{}, ErrInvalid
		}
		response, err := publicAssetClient(time.Minute).Do(request)
		if err != nil {
			return result{}, assetRequestFailure("GeoIP asset", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return result{}, assetResponseFailure("GeoIP asset", response.StatusCode)
		}
		file, err := os.CreateTemp("", "xnode-master-geoip-*.dat")
		if err != nil {
			return result{}, err
		}
		defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
		digest := sha256.New()
		size, err := io.Copy(io.MultiWriter(file, digest), io.LimitReader(response.Body, maxGeoIPAsset+1))
		if err != nil || size == 0 || size > maxGeoIPAsset {
			return result{}, ErrInvalid
		}
		hash := hex.EncodeToString(digest.Sum(nil))
		if config.GeoIPHash != "" && !strings.EqualFold(hash, config.GeoIPHash) {
			return result{}, ErrInvalid
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return result{}, err
		}
		prefixes, err := readGeoIPPrefixes(ctx, file, config.SourceGeoIP.Selectors)
		if err != nil {
			return result{}, err
		}
		generation := sha256.Sum256([]byte(hash + "\n" + strings.Join(config.SourceGeoIP.Selectors, "\n")))
		return result{prefixes, hex.EncodeToString(generation[:])}, nil
	})
	return value.prefixes, value.generation, err
}
