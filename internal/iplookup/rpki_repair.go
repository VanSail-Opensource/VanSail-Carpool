package iplookup

import (
	"context"
	"sync"
	"time"
)

// repairRPKI retains successful frozen pairs while refetching failed validations.
func (s *LiveService) repairRPKI(ctx context.Context, ip string, previous *RPKIResult) RPKIResult {
	if previous == nil || previous.Prefix == "" || len(previous.Routes) == 0 {
		return s.lookupRPKI(ctx, ip)
	}
	jobs := make(chan int, len(previous.Routes))
	for i, route := range previous.Routes {
		if liveMetaFailed(route.LiveMeta) {
			jobs <- i
		}
	}
	if len(jobs) == 0 {
		return s.lookupRPKI(ctx, ip)
	}
	close(jobs)
	result := *previous
	result.Routes = append([]RPKIRoute{}, previous.Routes...)
	result.Status, result.ErrorCode = "success", ""
	var work sync.WaitGroup
	for range min(2, len(previous.Routes)) {
		work.Add(1)
		go func() {
			defer work.Done()
			for i := range jobs {
				result.Routes[i] = s.validateRPKI(ctx, result.Prefix, result.Routes[i].ASN)
			}
		}()
	}
	work.Wait()
	failed := 0
	for _, route := range result.Routes {
		if liveMetaFailed(route.LiveMeta) {
			failed++
			result.Status = "partial"
		}
	}
	if failed == len(result.Routes) {
		result.Status, result.ErrorCode = "error", "IP_DETAILS_UNAVAILABLE"
	}
	result.ObservedAt = s.now().UTC().Format(time.RFC3339)
	return result
}
