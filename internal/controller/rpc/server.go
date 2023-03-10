package rpc

import (
	"context"
	"errors"
	"github.com/spch13/service-users-auth/internal/generated/rpc/auth"
	"github.com/spch13/service-users-auth/internal/model"
	"github.com/spch13/service-users-auth/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Server struct {
	auth.UnimplementedAuthServiceServer
	userRepo   *repository.UserRepositoryInMem
	jwtManager *JWTManager
}

func NewServer(manager *JWTManager, userRepo *repository.UserRepositoryInMem) *Server {
	return &Server{
		userRepo:   userRepo,
		jwtManager: manager,
	}
}

func (s *Server) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	user, err := s.userRepo.Find(req.GetUsername())
	if err != nil && errors.Is(err, repository.ErrNotFound) {
		return nil, status.Errorf(codes.Internal, "cannot find user: %v", err)
	}

	if user == nil || !user.IsCorrectPassword(req.GetPassword()) {
		return nil, status.Errorf(codes.NotFound, "incorrect username/password")
	}

	token, err := s.jwtManager.Generate(user)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot generate access token")
	}

	res := &auth.LoginResponse{AccessToken: token}
	return res, nil
}

func (s *Server) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	if req.Password != req.ConfirmPassword {
		return nil, status.Errorf(codes.InvalidArgument, "password fields are not match")
	}

	newUser, err := model.NewUser(req.Username, req.Password, "user")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error creating user, internal server error")
	}

	if err := s.userRepo.Save(newUser); err != nil {
		return nil, status.Errorf(codes.Internal, "error save to repo: %s", err)
	}

	token, err := s.jwtManager.Generate(newUser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot generate access token")
	}

	return &auth.RegisterResponse{
		AccessToken: token,
	}, nil
}

func (s *Server) RegisterAdmin(ctx context.Context, req *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	_, err := s.Register(ctx, req)
	if err != nil {
		return nil, err
	}

	err = s.userRepo.UpdateRole(req.Username, "admin")
	if err != nil {
		return nil, err
	}

	loginReq := &auth.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}

	resp, err := s.Login(ctx, loginReq)
	if err != nil {
		return nil, err
	}

	return &auth.RegisterResponse{
		AccessToken: resp.AccessToken,
	}, nil
}

func (s *Server) GetRole(ctx context.Context, req *auth.GetRoleRequest) (*auth.GetRoleResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		// No metadata in the context
		return &auth.GetRoleResponse{
			Role: "empty",
		}, nil
	}

	// Get the value of a specific metadata key
	role := md.Get("role")

	return &auth.GetRoleResponse{
		Role: role[0],
	}, nil
}

func (s *Server) UpdateRole(ctx context.Context, req *auth.UpdateRoleRequest) (*auth.UpdateRoleResponse, error) {
	err := s.userRepo.UpdateRole(req.Username, req.Role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error updating role: %v", err)
	}

	return &auth.UpdateRoleResponse{
		Message: "success",
	}, nil
}

func (s *Server) CheckProtect(context.Context, *auth.CheckProtectRequest) (*auth.CheckProtectResponse, error) {
	return &auth.CheckProtectResponse{
		Message: "ok",
	}, nil
}

func (s *Server) Healthcheck(context.Context, *auth.HealthCheckRequest) (*auth.HealthCheckResponse, error) {
	return &auth.HealthCheckResponse{
		Message: "ok",
	}, nil
}
