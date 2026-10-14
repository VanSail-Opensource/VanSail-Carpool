package billing

import (
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
)

const providerMessageLimit = 300

func providerCreateRejection(err error) (string, bool) {
	var apiError *bepusdt.APIError
	if !errors.As(err, &apiError) {
		return "", false
	}
	message := strings.Join(strings.Fields(apiError.Message), " ")
	runes := []rune(message)
	if len(runes) > providerMessageLimit {
		message = string(runes[:providerMessageLimit])
	}
	return message, true
}
