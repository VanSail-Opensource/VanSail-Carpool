package iplookup

import "errors"

func inventoryBuildError(err error) string {
	var coded *CodeError
	if errors.As(err, &coded) {
		return coded.Code
	}
	var sqliteCode interface{ Code() int }
	if errors.As(err, &sqliteCode) && sqliteCode.Code()&255 == 13 {
		return "IP_PROXY_LIMIT"
	}
	return "IP_LOOKUP_FAILED"
}
