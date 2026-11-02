package iplookup

import "errors"

func inventoryBuildError(err error) string {
	var coded *CodeError
	if errors.As(err, &coded) {
		return coded.Code
	}

	return "IP_LOOKUP_FAILED"
}
