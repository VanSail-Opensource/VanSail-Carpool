package subscription

import (
	"encoding/json"
	"errors"
)

// RetireTuning is only for decrypting pre-refinement documents. New writes use
// the strict ConnectionOverrides decoder and cannot reintroduce removed keys.
func RetireTuning(raw []byte) ([]byte, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, errors.New("invalid saved client tuning")
	}
	for _, key := range retiredTuningKeys {
		delete(values, key)
	}
	delete(values, "tcpMptcp") // The fixed-off policy is never a paid customization.
	return json.Marshal(values)
}

var retiredTuningKeys = []string{
	"muxConcurrency", "mihomoMuxOnlyTcp",
	"minVersion",
	"maxVersion",
	"enableSessionResumption",
	"disableSystemRoot",
	"useSystemCA",
	"cipherSuites",
	"xhttpNoGRPCHeader",
	"xmuxHKeepAlivePeriod",
	"domainStrategy",
	"tcpKeepAliveInterval",
	"tcpKeepAliveIdle",
	"tcpUserTimeout",
	"tcpCongestion",
	"tcpWindowClamp",
	"tcpMaxSeg",
	"interface",
	"mark",
	"penetrate",
	"happyPrioritizeIPv6",
	"happyTryDelayMs",
	"happyInterleave",
	"happyMaxConcurrentTry",
	"customSockopt",
	"udpFragment",
	"connectTimeoutMs",
	"downloadFingerprint",
	"downloadSpiderX",
	"downloadMinVersion",
	"downloadMaxVersion",
	"downloadEnableSessionResumption",
	"downloadDisableSystemRoot",
	"downloadUseSystemCA",
	"downloadCipherSuites",
	"downloadCurvePreferences",
	"downloadXhttpHeaders",
	"downloadXhttpXPaddingBytes",
	"downloadXhttpNoGRPCHeader",
	"downloadXmuxMaxConcurrency",
	"downloadXmuxMaxConnections",
	"downloadXmuxCMaxReuseTimes",
	"downloadXmuxHMaxRequestTimes",
	"downloadXmuxHMaxReusableSecs",
	"downloadXmuxHKeepAlivePeriod",
	"downloadDomainStrategy",
	"downloadTcpFastOpen",
	"downloadTcpKeepAliveInterval",
	"downloadTcpKeepAliveIdle",
	"downloadTcpUserTimeout",
	"downloadTcpCongestion",
	"downloadTcpWindowClamp",
	"downloadTcpMaxSeg",
	"downloadInterface",
	"downloadMark",
	"downloadTcpMptcp",
	"downloadHappyPrioritizeIPv6",
	"downloadHappyTryDelayMs",
	"downloadHappyInterleave",
	"downloadHappyMaxConcurrentTry",
	"downloadCustomSockopt",
}
