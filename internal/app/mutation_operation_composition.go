package app

import (
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func registerMutationOperationHandlers(dispatcher *providerops.Dispatcher, catalogService *catalog.Service,
	embyOperations *emby.OperationService, questionnaireService *questionnaires.Service, adminService *admin.Service) error {
	adminWorker, err := admin.NewMutationWorker(adminService)
	if err != nil {
		return err
	}
	registrations := []struct {
		kind    string
		handler providerops.Handler
	}{
		{providerops.KindSubscriptionRevoke, catalogService},
		{providerops.KindEmbySetup, embyOperations},
		{providerops.KindEmbyPreferences, embyOperations},
		{providerops.KindEmbyPassword, embyOperations},
		{providerops.KindEmbyProvisionRetry, embyOperations},
		{providerops.KindQuestionnaireSettlement, questionnaireService},
		{providerops.KindOutboxRetry, adminWorker},
		{providerops.KindPaymentRefund, adminWorker},
	}
	for _, registration := range registrations {
		if err := dispatcher.Register(registration.kind, registration.handler); err != nil {
			return fmt.Errorf("register %s operation: %w", registration.kind, err)
		}
	}
	return nil
}
