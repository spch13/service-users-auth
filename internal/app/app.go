package app

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"service-users-auth/internal/config"
	"service-users-auth/internal/controller/rpc"
	pb "service-users-auth/internal/generated/rpc/auth"
	"service-users-auth/internal/repository"
	"syscall"
	"time"
)

const (
	secretKey     = "secret"
	tokenDuration = 15 * time.Minute
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
	jwtManager := rpc.NewJWTManager(secretKey, tokenDuration)

	server := rpc.NewServer(jwtManager, userRepo)

	return &App{
		server:     server,
		cfg:        &cfg,
		jwtManager: jwtManager,
	}, nil
}

// [path.to.Method]roles
func accessibleRoles() map[string][]string {
	const authServicePath = "/service.AuthService/" // other service path to protect routes

	return map[string][]string{
		authServicePath + "CheckProtect": {"admin"},
	}
}

func (a *App) Run() error {
	lis, err := net.Listen("tcp", a.cfg.App.HOST+":"+a.cfg.App.PORT)
	if err != nil {
		return fmt.Errorf("error listen tcp port: %w", err)
	}

	// ==
	authInterceptor := rpc.NewAuthInterceptor(a.jwtManager, accessibleRoles())
	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(authInterceptor.Unary())),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(authInterceptor.Stream())),
	}

	grpcServer := grpc.NewServer(serverOptions...)
	pb.RegisterAuthServiceServer(grpcServer, a.server)
	// ===

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
