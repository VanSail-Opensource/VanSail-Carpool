package app

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"

func connectivityFailure(code string) error {
	return &connectivity.CodeError{Code: "CONNECTIVITY_" + code}
}
