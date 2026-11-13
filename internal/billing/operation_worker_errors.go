package billing

import (
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
)

func providerCreateRejection(err error) bool {
	var apiError *bepusdt.APIError
	return errors.As(err, &apiError)
}
