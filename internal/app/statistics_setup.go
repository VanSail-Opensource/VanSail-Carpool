package app

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
)

func registerStatisticsOperationHandler(dispatcher *providerops.Dispatcher, repository productstats.HostWorkerRepository, provider productstats.Provider) error {
	worker := productstats.NewHostRemarkWorker(repository, provider)
	return dispatcher.Register(providerops.KindHostRemarkUpdate, worker)
}
