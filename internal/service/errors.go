package service

import "errors"

var errAlreadyRunning = errors.New("another sinty-aegisd owns the name")
