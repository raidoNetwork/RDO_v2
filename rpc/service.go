package rpc

import (
	"context"
	"fmt"
	"net"
	runtimeDebug "runtime/debug"
	"strings"
	"sync"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/rpc/api"
	"github.com/raidoNetwork/RDO_v2/rpc/attestation"
	"github.com/raidoNetwork/RDO_v2/rpc/generator"
	"github.com/raidoNetwork/RDO_v2/rpc/rdochain"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var log = logrus.WithField("prefix", "RPC")

type Config struct {
	Host               string
	Port               string
	ChainService       api.ChainAPI
	AttestationService api.AttestationAPI
	GeneratorService   api.GeneratorAPI
	MaxMsgSize         int
}

type Service struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	cfg                 *Config
	listener            net.Listener
	grpcServer          *grpc.Server
	connectionMu        sync.RWMutex
	connectedRPCClients map[net.Addr]bool
	startFailure        error
}

func NewService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)

	srv := &Service{
		ctx:                 ctx,
		cancel:              cancel,
		cfg:                 cfg,
		connectedRPCClients: map[net.Addr]bool{},
	}

	return srv
}

// Start the gRPC server.
func (s *Service) Start() {
	address := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Errorf("Could not listen to port in Start() %s: %v", address, err)
		s.startFailure = err
		return
	}
	s.listener = lis
	log.WithField("address", address).Infof("gRPC server listening on %s", address)

	opts := []grpc.ServerOption{
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		middleware.WithUnaryServerChain(
			s.healthInterceptor,
			grpc_prometheus.UnaryServerInterceptor,
			s.recoveryInterceptor,
		),
		grpc.MaxRecvMsgSize(s.cfg.MaxMsgSize),
	}
	grpc_prometheus.EnableHandlingTimeHistogram()

	s.connectionMu.Lock()
	s.grpcServer = grpc.NewServer(opts...)
	s.connectionMu.Unlock()

	chainServer := &rdochain.Server{
		Server:      s.grpcServer,
		Backend:     s.cfg.ChainService,
		Attestation: s.cfg.AttestationService,
	}

	poolServer := &attestation.Server{
		Server:  s.grpcServer,
		Backend: s.cfg.AttestationService,
	}

	generatorServer := &generator.Server{
		Server:      s.grpcServer,
		Backend:     s.cfg.GeneratorService,
		Attestation: s.cfg.AttestationService,
	}

	prototype.RegisterRaidoChainServer(s.grpcServer, chainServer)
	prototype.RegisterAttestationServer(s.grpcServer, poolServer)
	prototype.RegisterGeneratorServer(s.grpcServer, generatorServer)

	go func() {
		if s.listener != nil {
			if err := s.grpcServer.Serve(s.listener); err != nil {
				log.Errorf("Could not serve gRPC: %v", err)
				s.startFailure = err
			}
		}
	}()
}

// Stop the service.
func (s *Service) Stop() error {
	log.Info("Stop gRPC service")

	s.cancel()
	if s.listener != nil {
		s.connectionMu.Lock()
		s.grpcServer.GracefulStop()
		s.connectionMu.Unlock()
		log.Debug("Initiated graceful stop of gRPC server")
	}
	return nil
}

// Status returns nil or credentialError
func (s *Service) Status() error {
	if s.startFailure != nil {
		return s.startFailure
	}

	return nil
}

// healthInterceptor creates UnaryServerInterceptor wrap for given callback function
func (s *Service) healthInterceptor(ctx context.Context,
	req interface{},
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	srvStatus, _ := s.cfg.ChainService.GetServiceStatus()
	if strings.Contains(srvStatus, "Not ready") {
		return nil, status.Error(14, srvStatus)
	}

	s.logNewClientConnection(ctx)
	return handler(ctx, req)
}

// logNewClientConnection
func (s *Service) logNewClientConnection(ctx context.Context) {
	if clientInfo, ok := peer.FromContext(ctx); ok {
		// Check if we have not yet observed this grpc client connection
		// in the running node.
		s.connectionMu.Lock()
		defer s.connectionMu.Unlock()

		if !s.connectedRPCClients[clientInfo.Addr] {
			log.WithFields(logrus.Fields{
				"addr": clientInfo.Addr.String(),
			}).Infof("New gRPC client connected to raido node")
			s.connectedRPCClients[clientInfo.Addr] = true
		}
	}
}

func (s *Service) recoveryInterceptor(ctx context.Context,
	req interface{},
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (res interface{}, err error) {
	defer func() {
		if x := recover(); x != nil {
			log.Errorf("gRPC handler panic: %v\n%v", x, string(runtimeDebug.Stack()))
			err = status.Error(17, "Server Internal Error")
		}
	}()

	return handler(ctx, req)
}
