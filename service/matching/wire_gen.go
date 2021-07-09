// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package matching

import (
	"go.temporal.io/server/common/config"
	"go.temporal.io/server/common/dynamicconfig"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/metrics"
	"go.temporal.io/server/common/resource"
)

// Injectors from wire.go:

// todomigryz: implement this method. Replace NewService method.
// todomigryz: Need to come up with proper naming convention for initialize vs factory methods.
func InitializeMatchingService(logger log.Logger, params *resource.BootstrapParams, dcClient dynamicconfig.Client, metricsReporter metrics.Reporter, svcCfg config.Service) (*Service, error) {
	taggedLogger, err := TaggedLoggerProvider(logger)
	if err != nil {
		return nil, err
	}
	matchingConfig, err := ServiceConfigProvider(logger, dcClient)
	if err != nil {
		return nil, err
	}
	throttledLogger, err := ThrottledLoggerProvider(taggedLogger, matchingConfig)
	if err != nil {
		return nil, err
	}
	matchingMetricsReporter, err := MetricsReporterProvider(taggedLogger, metricsReporter, svcCfg)
	if err != nil {
		return nil, err
	}
	serviceIdx := ServiceIdxProvider()
	client, err := MetricsClientProvider(taggedLogger, matchingMetricsReporter, serviceIdx)
	if err != nil {
		return nil, err
	}
	service, err := NewService(params, logger, taggedLogger, throttledLogger, matchingConfig, client)
	if err != nil {
		return nil, err
	}
	return service, nil
}
