package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func (a remnaAdapter) RequestConnectionScan(ctx context.Context, remoteID string) (string, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return "", err
	}
	return remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (string, error) {
		return client.RequestUserConnections(callCtx, userID)
	})
}

func (a remnaAdapter) PollConnectionScan(ctx context.Context, jobID string) (connections.ProviderScan, error) {
	result, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (remnawave.ConnectionScan, error) {
		return client.UserConnections(callCtx, jobID)
	})
	if err != nil {
		return connections.ProviderScan{}, err
	}
	nodes := make([]connections.ProviderNode, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		ips := make([]connections.Observation, 0, len(node.IPs))
		for _, item := range node.IPs {
			ips = append(ips, connections.Observation{Address: item.IP, LastSeen: item.LastSeen})
		}
		nodes = append(nodes, connections.ProviderNode{UUID: node.UUID, Name: node.Name, CountryCode: node.CountryCode, IPs: ips})
	}
	return connections.ProviderScan{Completed: result.Completed, Failed: result.Failed, ProgressPercent: result.Progress, Nodes: nodes}, nil
}

func (a remnaAdapter) DropConnection(ctx context.Context, ip, nodeUUID string) error {
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		return client.DropConnectionByIP(callCtx, ip, nodeUUID)
	})
}

func (a remnaAdapter) BlockIP(ctx context.Context, ip, nodeUUID string, timeoutSeconds int) error {
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		return client.BlockIP(callCtx, ip, nodeUUID, timeoutSeconds)
	})
}

func (a remnaAdapter) UnblockIP(ctx context.Context, ip, nodeUUID string) error {
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		return client.UnblockIP(callCtx, ip, nodeUUID)
	})
}

func (a remnaAdapter) MemberOperationState(ctx context.Context, remoteID string) (purchaseops.RemoteState, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(callCtx, userID)
	})
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	return purchaseops.RemoteState{UsedTrafficBytes: user.UserTraffic.UsedTrafficBytes, LastTrafficResetAt: user.LastTrafficResetAt,
		Quiesced: user.Status == remnawave.UserStatusDisabled}, nil
}

func (a remnaAdapter) QuiesceMemberOperation(ctx context.Context, remoteID string) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	status := remnawave.UserStatusDisabled
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		_, callErr := client.UpdateUser(callCtx, remnawave.UpdateUserRequest{ID: userID, Status: &status})
		return callErr
	})
}

func (a remnaAdapter) RestoreMemberOperation(ctx context.Context, remoteID string, purchase model.Purchase) error {
	return a.ApplyEntitlement(ctx, remoteID, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.SquadUUIDs, purchase.ValidUntil)
}

func (a remnaAdapter) DefinitiveMutationFailure(err error) bool {
	var apiError *remnawave.APIError
	return errors.As(err, &apiError) && apiError.HTTPStatus >= http.StatusBadRequest && apiError.HTTPStatus < http.StatusInternalServerError
}
