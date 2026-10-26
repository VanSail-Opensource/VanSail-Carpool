package pricing

import (
	"context"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

type ResponseObservation = compute.ResponseObservation
type ResponseModel = compute.ResponseModel

// FitResponse exposes the shared pure kernel; application analytics use the durable lane.
func FitResponse(rows []ResponseObservation) ResponseModel {
	m, _ := compute.FitResponse(context.Background(), rows)
	return m
}
