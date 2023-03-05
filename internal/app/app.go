package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spch13/service-users-auth/internal/config"
	"github.com/spch13/service-users-auth/internal/controller/rpc"
	pb "github.com/spch13/service-users-auth/internal/generated/rpc/auth"
	"github.com/spch13/service-users-auth/internal/repository"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	tokenDuration = 7 * 24 * time.Hour
)

type App struct {
	server     *rpc.Server
	jwtManager *rpc.JWTManager
	cfg        *config.Config
}

func New() (*App, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepoInMem()
	jwtManager := rpc.NewJWTManager(cfg.Secret.SecretKey, tokenDuration)

	server := rpc.NewServer(jwtManager, userRepo)

	return &App{
		server:     server,
		cfg:        &cfg,
		jwtManager: jwtManager,
	}, nil
}

const authServicePath = "/service.AuthService/" // other service path to protect routes

// [path.to.Method]roles
func accessibleRoles() map[string][]string {
	return map[string][]string{
		authServicePath + "CheckProtect": {"admin"},
		authServicePath + "UpdateRole":   {"admin"},
	}
}

func routeGetRole() map[string]struct{} {
	return map[string]struct{}{
		authServicePath + "GetRole": {},
	}
}

func (a *App) Run() error {
	lis, err := net.Listen("tcp", a.cfg.App.HOST+":"+a.cfg.App.PORT)
	if err != nil {
		return fmt.Errorf("error listen tcp port: %w", err)
	}

	authInterceptor := rpc.NewAuthInterceptor(a.jwtManager, accessibleRoles())
	roleInterceptor := rpc.NewRoleInterceptor(a.jwtManager, routeGetRole())

	serverOptions := []grpc.ServerOption{
		grpc.Creds(insecure.NewCredentials()),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_opentracing.StreamServerInterceptor(),
			grpc_recovery.StreamServerInterceptor(),
			grpc_middleware.ChainStreamServer(authInterceptor.Stream()),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_opentracing.UnaryServerInterceptor(),
			grpc_recovery.UnaryServerInterceptor(),
			grpc_middleware.ChainUnaryServer(authInterceptor.Unary()),
			grpc_middleware.ChainUnaryServer(roleInterceptor.Unary()),
		)),
	}
	grpcServer := grpc.NewServer(serverOptions...)
	pb.RegisterAuthServiceServer(grpcServer, a.server)
	reflection.Register(grpcServer)

	eChan := make(chan error)
	interrupt := make(chan os.Signal, 1)

	log.Printf("grpc server has been started on port: %s", a.cfg.App.PORT)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			eChan <- fmt.Errorf("listen and serve grpc: %w", err)
		}
	}()

	log.Printf("auth gateway server has been started on port: %s", a.cfg.App.GWPORT)
	go func() {
		if err := a.startHTTP(); err != nil {
			eChan <- fmt.Errorf("listen and server gateway: %w", err)
		}
	}()

	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-eChan:
		return fmt.Errorf("start grpc service: %w", err)
	case <-interrupt:
		grpcServer.GracefulStop()
		log.Printf("graceful stopping...")
	}

	return nil

}

func (a *App) startHTTP() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register grpc-gateway
	gwmux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := pb.RegisterAuthServiceHandlerFromEndpoint(ctx, gwmux, ":"+a.cfg.App.PORT, opts)
	if err != nil {
		return err
	}

	// server gateway
	mux := http.NewServeMux()
	mux.Handle("/", gwmux)

	// serve swagger
	fs := http.FileServer(http.Dir("./swaggerui"))
	mux.Handle("/swaggerui/", http.StripPrefix("/swaggerui/", fs))

	port := a.cfg.App.GWPORT
	err = http.ListenAndServe(":"+port, mux) // swagger service
	if err != nil {
		return err
	}

	return nil
}
