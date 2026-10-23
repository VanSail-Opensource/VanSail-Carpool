package app

import (
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	platformoutbox "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

type memberWorkflowServices struct {
	connections *connections.Service
	drops       *connections.DropService
	purchases   *purchaseops.Service
}

func newMemberWorkflows(store *database.Store, proxy proxyProvider, vault *secret.Vault, signingKey []byte) (memberWorkflowServices, *connections.Worker, *connections.ExpiryWorker, *providerops.Dispatcher, error) {
	signer, err := connections.NewSigner(signingKey)
	if err != nil {
		return memberWorkflowServices{}, nil, nil, nil, err
	}
	secrets := emby.NewSecretBox(vault)
	services := memberWorkflowServices{
		connections: connections.NewService(store, proxy, signer),
		drops:       connections.NewDropService(store, signer, secrets),
		purchases:   purchaseops.NewService(store, proxy),
	}
	dispatcher := providerops.NewDispatcher(store)
	purchaseWorker := purchaseops.NewWorker(store, proxy)
	for _, kind := range []string{purchaseops.OperationResetKind, purchaseops.OperationRefundKind} {
		if err := dispatcher.Register(kind, purchaseWorker); err != nil {
			return memberWorkflowServices{}, nil, nil, nil, fmt.Errorf("register %s: %w", kind, err)
		}
	}
	if err := dispatcher.Register(connections.DropOperationKind, connections.NewDropWorker(store, proxy, signer, secrets)); err != nil {
		return memberWorkflowServices{}, nil, nil, nil, fmt.Errorf("register connection drop: %w", err)
	}
	if err := dispatcher.Register(connections.BlockOperationKind, connections.NewBlockWorker(store, proxy, secrets)); err != nil {
		return memberWorkflowServices{}, nil, nil, nil, fmt.Errorf("register connection block: %w", err)
	}
	if err := dispatcher.Register(connections.UnblockOperationKind, connections.NewUnblockWorker(store, proxy, secrets)); err != nil {
		return memberWorkflowServices{}, nil, nil, nil, fmt.Errorf("register connection unblock: %w", err)
	}
	return services, connections.NewWorker(store, proxy), connections.NewExpiryWorker(store, proxy, secrets), dispatcher, nil
}

func registerAdminOperationHandlers(dispatcher *providerops.Dispatcher, store *database.Store, proxy proxyProvider) error {
	worker := admin.NewUserOperationWorker(store, proxy)
	for _, kind := range []string{providerops.KindAdminEntitlementEdit, providerops.KindAdminEntitlementRefund,
		providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension, providerops.KindNodeCompensation} {
		if err := dispatcher.Register(kind, worker); err != nil {
			return err
		}
	}
	providerWorker := admin.NewUserProviderWorker(store, proxy)
	for _, kind := range []string{providerops.KindAdminTemporaryBan, providerops.KindAdminTemporaryUnban} {
		if err := dispatcher.Register(kind, providerWorker); err != nil {
			return err
		}
	}
	return nil
}

func registerProviderDispatcher(worker *platformoutbox.Worker, dispatcher *providerops.Dispatcher) error {
	return worker.Register(providerops.OutboxKind, dispatcher)
}

func registerComboControlHandlers(dispatcher *providerops.Dispatcher, store *database.Store, synchronizer purchaseops.EntitlementSynchronizer, onSynced func(string)) error {
	handler := purchaseops.NewComboControlWorker(store, synchronizer, onSynced)
	for _, kind := range []string{purchaseops.OperationSquadSwitch, purchaseops.OperationEarlyActivation} {
		if err := dispatcher.Register(kind, handler); err != nil {
			return err
		}
	}
	return nil
}
