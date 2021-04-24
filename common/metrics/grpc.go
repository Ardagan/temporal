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

package metrics

import (
	"context"

	metricspb "go.temporal.io/server/api/metrics/v1"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/log/tag"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type metricsContextKey struct {}

var (
	baggageTrailerKey = "metrics-baggage-bin"
	metricsCtxKey = metricsContextKey{}
)

// NewServerMetricsContextInjectorInterceptor returns grpc server interceptor that wraps golang context into golang
// metrics propagation context.
func NewServerMetricsContextInjectorInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctxWithMetricsBaggage := AddMetricsBaggageToContext(ctx)
		return handler(ctxWithMetricsBaggage, req)
	}
}

// NewClientMetricsTrailerPropagatorInterceptor returns grpc client interceptor that injects metrics propagation context
// received in trailer into golang metrics propagation context.
func NewClientMetricsTrailerPropagatorInterceptor(logger log.Logger) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		var trailer metadata.MD
		optsWithTrailer := append(opts, grpc.Trailer(&trailer))
		err := invoker(ctx, method, req, reply, cc, optsWithTrailer...)

		metricsBaggageStrings := trailer.Get(baggageTrailerKey)
		if metricsBaggageStrings == nil {
			return err
		}

		for _, str := range metricsBaggageStrings {
			data := []byte(str)
			metricsBaggage := &metricspb.Baggage{}
			unmarshalErr := metricsBaggage.Unmarshal(data)
			if unmarshalErr != nil {
				logger.Error("unable to unmarshal metrics baggage from trailer", tag.Error(unmarshalErr))
				continue
			}
			for key, value := range metricsBaggage.CountersInt {
				ContextCounterAdd(ctx, logger, key, value)
			}
		}

		return err
	}
}

// NewServerMetricsTrailerPropagatorInterceptor returns grpc server interceptor that injects metrics propagation context
// into gRPC trailer.
func NewServerMetricsTrailerPropagatorInterceptor(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// we want to return original handler response, so don't override err
		resp, err := handler(ctx, req)

		baggage := GetMetricsBaggageFromContext(ctx)
		if baggage == nil {
			return resp, err
		}

		bytes, marshalErr := baggage.Marshal()
		if marshalErr != nil {
			logger.Error("unable to marshal metric baggage", tag.Error(marshalErr))
		}

		md := metadata.Pairs(baggageTrailerKey, string(bytes))
		marshalErr = grpc.SetTrailer(ctx, md)
		if marshalErr != nil {
			logger.Error("unable to add metrics baggage to gRPC trailer", tag.Error(marshalErr))
		}

		return resp, err
	}
}

// GetMetricsBaggageFromContext extracts propagation context from go context.
func GetMetricsBaggageFromContext(ctx context.Context) *metricspb.Baggage {
	metricValues := ctx.Value(metricsCtxKey)
	if metricValues == nil {
		return nil
	}

	metricValuesCtx, ok := metricValues.(*metricspb.Baggage)
	if !ok {
		return nil
	}
	return metricValuesCtx

}

func AddMetricsBaggageToContext(ctx context.Context) context.Context {
	metricsBaggage := &metricspb.Baggage{CountersInt: make(map[string]int64)}
	return context.WithValue(ctx, metricsCtxKey, metricsBaggage)
}

// ContextCounterAdd adds value to counter within propagation context.
func ContextCounterAdd(ctx context.Context, logger log.Logger, name string, value int64) {
	metricsBaggage := GetMetricsBaggageFromContext(ctx)

	if metricsBaggage == nil {
		logger.Error("unable to fetch metrics baggage from context")
		return
	}

	metricValue, _ := metricsBaggage.CountersInt[name]
	metricValue += value
	metricsBaggage.CountersInt[name] = metricValue
}
