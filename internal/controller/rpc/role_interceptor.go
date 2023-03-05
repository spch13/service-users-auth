package rpc

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
)

var SkipMethod = errors.New("skip method")

type RoleInterceptor struct {
	jwtManager *JWTManager
	routes     map[string]struct{}
}

// NewAuthInterceptor returns a new auth interceptor
func NewRoleInterceptor(jwtManager *JWTManager, routes map[string]struct{}) *RoleInterceptor {
	return &RoleInterceptor{jwtManager, routes}
}

// Unary returns a server interceptor function to authenticate and authorize unary RPC
func (interceptor *RoleInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		log.Println("--> unary interceptor: ", info.FullMethod)
		claims, err := interceptor.getUserClaims(ctx, info.FullMethod)
		if err != nil {
			if errors.Is(err, SkipMethod) {
				return handler(ctx, req)
			}
			return nil, err
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		// Add a key-value pair to the metadata.
		md.Set("role", claims.Role)
		md.Set("username", claims.Username)

		// Add the metadata to the context.
		newCtx := metadata.NewIncomingContext(ctx, md)

		return handler(newCtx, req)
	}
}

func (in *RoleInterceptor) getUserClaims(ctx context.Context, method string) (*UserClaims, error) {
	_, ok := in.routes[method]
	if !ok {
		// everyone can access ! - method are not protected - ok
		return nil, SkipMethod
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}

	accessToken := values[0]
	claims, err := in.jwtManager.Verify(accessToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "access token is invalid: %v", err)
	}

	return claims, nil
}
