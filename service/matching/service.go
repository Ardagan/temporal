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

package matching

import (
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/uber-go/tally"
	"github.com/uber/tchannel-go"
	"go.temporal.io/server/client"
	"go.temporal.io/server/common/cache"
	"go.temporal.io/server/common/membership"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.temporal.io/server/api/matchingservice/v1"
	"go.temporal.io/server/common"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/log/tag"
	"go.temporal.io/server/common/metrics"
	persistenceClient "go.temporal.io/server/common/persistence/client"
)

// Service represents the matching service
type (
	Service struct {
		logger log.Logger

		status  int32
		handler *Handler
		config  *Config

		server *grpc.Server

		metricsScope           tally.Scope
		runtimeMetricsReporter *metrics.RuntimeMetricsReporter
		membershipMonitor      membership.Monitor
		namespaceCache         cache.NamespaceCache
		persistenceBean        persistenceClient.Bean
		ringpopChannel         *tchannel.Channel
		grpcListener           net.Listener
		clientBean             client.Bean // needed for onebox. Should remove if possible.
	}

	TaggedLogger log.Logger
	GRPCListener net.Listener
)

// NewService builds a new matching service
func NewService(
	taggedLogger TaggedLogger,
	throttledLogger log.ThrottledLogger,
	serviceConfig *Config,
	persistenceBean persistenceClient.Bean,
	namespaceCache cache.NamespaceCache,
	grpcServer *grpc.Server,
	grpcListener GRPCListener,
	membershipMonitor membership.Monitor,
	clientBean client.Bean,
	ringpopChannel *tchannel.Channel,
	handler *Handler,
	runtimeMetricsReporter *metrics.RuntimeMetricsReporter,
	deprecatedTally tally.Scope,
) (*Service, error) {

	return &Service{
		status:                 common.DaemonStatusInitialized,
		config:                 serviceConfig,
		server:                 grpcServer,
		handler:                handler,
		logger:                 taggedLogger,
		metricsScope:           deprecatedTally,
		runtimeMetricsReporter: runtimeMetricsReporter,
		membershipMonitor:      membershipMonitor,
		namespaceCache:         namespaceCache,
		persistenceBean:        persistenceBean,
		ringpopChannel:         ringpopChannel,
		grpcListener:           grpcListener,
		clientBean:             clientBean,
	}, nil
}

func (s *Service) GetClientBean() client.Bean {
	return s.clientBean
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	logger := s.logger
	logger.Info("matching starting")

	s.metricsScope.Counter(metrics.RestartCount).Inc(1)
	s.runtimeMetricsReporter.Start()

	s.membershipMonitor.Start()
	s.namespaceCache.Start()

	hostInfo, err := s.membershipMonitor.WhoAmI()
	if err != nil {
		s.logger.Fatal("fail to get host info from membership monitor", tag.Error(err))
	}

	// The service is now started up
	s.logger.Info("Service resources started", tag.Address(hostInfo.GetAddress()))

	// seed the random generator once for this service
	rand.Seed(time.Now().UnixNano())

	s.handler.Start()

	matchingservice.RegisterMatchingServiceServer(s.server, s.handler)
	healthpb.RegisterHealthServer(s.server, s.handler)

	listener := s.grpcListener
	logger.Info("Starting to serve on matching listener")
	if err := s.server.Serve(listener); err != nil {
		logger.Fatal("Failed to serve on matching listener", tag.Error(err))
	}
}

// Stop stops the service
func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(
		&s.status,
		common.DaemonStatusStarted,
		common.DaemonStatusStopped,
	) {
		return
	}

	// remove self from membership ring and wait for traffic to drain
	s.logger.Info("ShutdownHandler: Evicting self from membership ring")
	s.membershipMonitor.EvictSelf()
	s.logger.Info("ShutdownHandler: Waiting for others to discover I am unhealthy")
	time.Sleep(s.config.ShutdownDrainDuration())

	// TODO: Change this to GracefulStop when integration tests are refactored.
	s.server.Stop()

	s.handler.Stop()

	s.namespaceCache.Stop()
	s.membershipMonitor.Stop()
	s.ringpopChannel.Close() // todo: we do not start this in Start(), need to update initialization and ownership.
	s.runtimeMetricsReporter.Stop()
	s.persistenceBean.Close() // todo: we do not start this in Start(), need to update initialization and ownership.

}
