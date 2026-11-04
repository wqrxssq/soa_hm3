package svcjwt

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var currentTime = time.Now

func UnaryClientInterceptor(secret string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token, err := Sign(secret, currentTime())
		if err != nil {
			return status.Error(codes.Internal, "не удалось подписать вызов")
		}
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataKey, "Bearer "+token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func UnaryServerInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		token, ok := tokenFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "нет учётных данных")
		}
		if err := Parse(secret, token, currentTime()); err != nil {
			return nil, status.Error(codes.Unauthenticated, "недействительный токен")
		}
		return handler(ctx, req)
	}
}

func tokenFromContext(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get(MetadataKey)
	if len(values) == 0 {
		return "", false
	}
	const prefix = "Bearer "
	value := values[0]
	if len(value) <= len(prefix) || !strings.HasPrefix(value, prefix) {
		return "", false
	}
	return value[len(prefix):], true
}
