package xnode_test

// Credit production tests use synthetic policies and isolate the durable credit
// ledger. This test-only opt-in does not claim Store records hourly history;
// production Store must implement that atomic writer before adding the method.
func (*hourlyProductionRepository) HourlyTrafficHistoryReady() bool { return true }
