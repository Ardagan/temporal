// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package temporal

import (
	"github.com/google/wire"
	"github.com/urfave/cli/v2"
	"go.temporal.io/server/common/log"
)

// Injectors from wire.go:

func InitializeServer(c *cli.Context) (*Server, error) {
	config, err := DefaultConfigProvider(c)
	if err != nil {
		return nil, err
	}
	logger := DefaultLogger(config)
	serviceNamesList := DefaultServiceNameListProvider(logger, c)
	authorizer, err := DefaultAuthorizerProvider(config)
	if err != nil {
		return nil, err
	}
	claimMapper, err := DefaultClaimMapper(config, logger)
	if err != nil {
		return nil, err
	}
	client := DefaultDynamicConfigClientProvider(config, logger)
	collection := DefaultDynamicConfigCollectionProvider(client, logger)
	abstractDataStoreFactory := DefaultDatastoreFactory()
	metricsReporters, err := DefaultMetricsReportersProvider(config, logger)
	if err != nil {
		return nil, err
	}
	tlsConfigProvider, err := DefaultTLSConfigProvider(config, logger, metricsReporters)
	if err != nil {
		return nil, err
	}
	jwtAudienceMapper := DefaultAudienceGetterProvider()
	serviceResolver := DefaultPersistenseServiceResolverProvider()
	visibilityWritingMode, err := AdvancedVisibilityWritingModeProvider(config, collection)
	if err != nil {
		return nil, err
	}
	dataStore, err := AdvancedVisibilityStoreProvider(config, visibilityWritingMode)
	if err != nil {
		return nil, err
	}
	elasticsearch := ESConfigProvider(dataStore)
	esHttpClient, err := DefaultElasticSearchHttpClientProvider(elasticsearch)
	if err != nil {
		return nil, err
	}
	clientClient, err := ESClientProvider(config, logger, esHttpClient, serviceResolver, elasticsearch, visibilityWritingMode)
	if err != nil {
		return nil, err
	}
	servicesProviderDeps := &ServicesProviderDeps{
		cfg:                        config,
		services:                   serviceNamesList,
		logger:                     logger,
		namespaceLogger:            logger,
		authorizer:                 authorizer,
		claimMapper:                claimMapper,
		dynamicConfigClient:        client,
		dynamicConfigCollection:    collection,
		customDatastoreFactory:     abstractDataStoreFactory,
		metricReporters:            metricsReporters,
		tlsConfigProvider:          tlsConfigProvider,
		audienceGetter:             jwtAudienceMapper,
		persistenceServiceResolver: serviceResolver,
		esConfig:                   elasticsearch,
		esClient:                   clientClient,
	}
	v, err := ServicesProvider(servicesProviderDeps)
	if err != nil {
		return nil, err
	}
	serverInterruptCh := DefaultInterruptChProvider()
	server, err := ServerProvider(logger, config, serviceNamesList, v, serviceResolver, abstractDataStoreFactory, client, collection, serverInterruptCh)
	if err != nil {
		return nil, err
	}
	return server, nil
}

// wire.go:

func InitializeDefaultUserProviderSet(c *cli.Context) wire.ProviderSet {
	return wire.NewSet(
		DefaultConfigProvider,
		DefaultLogger,
		DefaultDynamicConfigClientProvider,
		DefaultAuthorizerProvider,
		DefaultClaimMapper,
		DefaultServiceNameListProvider,
		DefaultDatastoreFactory,
		DefaultMetricsReportersProvider,
		DefaultTLSConfigProvider,
		DefaultDynamicConfigCollectionProvider,
		DefaultAudienceGetterProvider,
		DefaultPersistenseServiceResolverProvider,
		DefaultElasticSearchHttpClientProvider,
	)
}

var UserSet = wire.NewSet(
	DefaultConfigProvider,
	DefaultLogger, wire.Bind(new(NamespaceLogger), new(log.Logger)), DefaultDynamicConfigClientProvider,
	DefaultAuthorizerProvider,
	DefaultClaimMapper,
	DefaultServiceNameListProvider,
	DefaultDatastoreFactory,
	DefaultMetricsReportersProvider,
	DefaultTLSConfigProvider,
	DefaultDynamicConfigCollectionProvider,
	DefaultAudienceGetterProvider,
	DefaultPersistenseServiceResolverProvider,
	DefaultElasticSearchHttpClientProvider,
	DefaultInterruptChProvider,
)

var serverSet = wire.NewSet(
	ServicesProvider,
	ServerProvider,
	AdvancedVisibilityStoreProvider,
	ESClientProvider,
	ESConfigProvider,
	AdvancedVisibilityWritingModeProvider, wire.Struct(new(ServicesProviderDeps), "*"),
)
