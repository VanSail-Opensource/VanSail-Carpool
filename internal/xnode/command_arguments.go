package xnode

import (
	"context"
	"time"
)

func (s *Service) commandArguments(ctx context.Context, node string, request CommandRequest) (map[string]any, error) {
	args := map[string]any{}
	switch request.Name {
	case "ddns.run":
		args["job_id"] = request.JobID
	case "update.install":
		if request.Version != "" {
			args["version"] = request.Version
		}
	case "speedtest":
		endpoints, err := s.Endpoints(ctx)
		if err != nil {
			return nil, err
		}
		var peer Endpoint
		found := false
		for _, endpoint := range endpoints {
			if endpoint.ID == request.PeerEndpointID && endpoint.Enabled && endpoint.NodeID != node {
				peer = endpoint
				found = true
				break
			}
		}
		if !found {
			return nil, ErrInvalid
		}
		fleet, err := s.Global(ctx)
		if err != nil {
			return nil, err
		}
		if fleet.SpeedtestDownloadURL == "" || fleet.SpeedtestUploadURL == "" {
			return nil, ErrInvalid
		}
		identity, err := s.AuthorizeService(ctx, "peer-speedtest", peer, s.now().Add(5*time.Minute))
		if err != nil {
			return nil, err
		}
		if err = s.master.Sync(ctx, peer.NodeID); err != nil {
			return nil, err
		}
		if err = s.master.WaitApplied(ctx, peer.NodeID); err != nil {
			return nil, err
		}
		proxy, err := s.endpointProxy(ctx, identity, peer)
		if err != nil {
			return nil, err
		}
		outbound, err := proxy.Outbound()
		if err != nil {
			return nil, err
		}
		args = map[string]any{"peer_outbound": outbound, "download_url": fleet.SpeedtestDownloadURL, "upload_url": fleet.SpeedtestUploadURL, "max_bytes": 32 << 20, "timeout_seconds": 60}
	}
	return args, nil
}
