// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

// +build !test

package api

import (
	"fmt"
	"time"

	log "github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mfxkit/mfxkit"
)

var _ mfxkit.Service = (*loggingMiddleware)(nil)

type loggingMiddleware struct {
	logger log.Logger
	svc    mfxkit.Service
}

// LoggingMiddleware adds logging facilities to the core service.
func LoggingMiddleware(svc mfxkit.Service, logger log.Logger) mfxkit.Service {
	return &loggingMiddleware{logger, svc}
}

func (lm *loggingMiddleware) Ping(secret string) (response string, err error) {
	defer func(begin time.Time) {
		message := fmt.Sprintf("Method ping for secret %s took %s to complete", secret, time.Since(begin))
		if err != nil {
			lm.logger.Warn(fmt.Sprintf("%s with error: %s.", message, err))
			return
		}
		lm.logger.Info(fmt.Sprintf("%s without errors.", message))
	}(time.Now())

	return lm.svc.Ping(secret)
}
