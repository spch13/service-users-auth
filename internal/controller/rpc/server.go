package rpc

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"service-users-auth/internal/generated/rpc/auth"
	"service-users-auth/internal/model"
	"service-users-auth/internal/repository"
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
		return nil, status.Errorf(codes.Internal, "error: %s", err)
	}

	token, err := s.jwtManager.Generate(newUser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot generate access token")
	}

	return &auth.RegisterResponse{
		AccessToken: token,
	}, nil
}

func (s *Server) CheckProtect(context.Context, *auth.CheckProtectRequest) (*auth.CheckProtectResponse, error) {
	return &auth.CheckProtectResponse{
		Message: "ok",
	}, nil
}
