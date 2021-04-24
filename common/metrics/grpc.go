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

type metricsContextMetadata struct {
	trailerID string
}

var (
	metricsContextMd = metricsContextMetadata{trailerID: "metrics-context-bin"}
)

// NewClientMetricsTrailerPropagatorInterceptor returns grpc client interceptor that injects metrics propagation context
// received in trailer into golang metrics propagation context.
func NewClientMetricsTrailerPropagatorInterceptor(logger log.Logger) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		var md metadata.MD
		optsWithTrailer := append(opts, grpc.Trailer(&md))
		err := invoker(ctx, method, req, reply, cc, optsWithTrailer...)

		metricUpdates := md.Get(metricsContextMd.trailerID)
		if metricUpdates == nil {
			return err
		}

		for _, str := range metricUpdates {
			data := []byte(str)
			propagationContext := metricspb.Baggage{}
			unmarshalError := propagationContext.Unmarshal(data)
			if unmarshalError != nil {
				logger.Error("unable to unmarshal metrics propagation data from trailer", tag.Error(unmarshalError))
				continue
			}
			for key, value := range propagationContext.CountersInt {
				ContextCounterAdd(ctx, logger, key, value)
			}
		}

		return err
	}
}

// NewServerMetricsContextInjectorInterceptor returns grpc server interceptor that wraps golang context into golang
// metrics propagation context.
func NewServerMetricsContextInjectorInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		metricValues := &metricspb.Baggage{CountersInt: make(map[string]int64)}
		ctxWithMetricValues := context.WithValue(ctx, metricsContextMd, metricValues)
		return handler(ctxWithMetricValues, req)
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
		// we want to return original handler responce, so don't override err
		resp, err := handler(ctx, req)

		propagationContext := GetPropagationContextFromGoContext(ctx)
		if propagationContext == nil {
			return resp, err
		}

		bytes, error := propagationContext.Marshal()
		if error != nil {
			logger.Error("unable to marshal metricValues", tag.Error(error))
		}

		md := metadata.Pairs(metricsContextMd.trailerID, string(bytes))
		error = grpc.SetTrailer(ctx, md)
		if error != nil {
			logger.Error("unable to set metrics propagation context gRPC trailer", tag.Error(error))
		}

		return resp, err
	}
}

// GetPropagationContextFromGoContext extracts propagation context from go context.
func GetPropagationContextFromGoContext(ctx context.Context) *metricspb.Baggage {
	metricValues := ctx.Value(metricsContextMd)
	if metricValues == nil {
		return nil
	}

	metricValuesCtx, ok := metricValues.(*metricspb.Baggage)
	if !ok {
		return nil
	}
	return metricValuesCtx

}

// ContextCounterAdd adds value to counter within propagation context.
func ContextCounterAdd(ctx context.Context, logger log.Logger, name string, value int64) {
	propagationContext := GetPropagationContextFromGoContext(ctx)
	if propagationContext == nil {
		logger.Error("unable to fetch metricsPropagationContext from context")
		return
	}
	if metricValue, ok := propagationContext.CountersInt[name]; ok {
		metricValue += value
	} else {
		propagationContext.CountersInt[name] = value
	}
}
