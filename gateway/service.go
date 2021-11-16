package gateway

import (
	"context"
	"fmt"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"net/http"
	"sync"
	"time"
)

type ChainAPI interface {
	consensus.BalanceReader

	GetSyncStatus() (string, error)
	GetServiceStatus() (string, error)
}

type AttestationAPI interface {
	SendRawTx(transaction *prototype.Transaction) error
	GetServiceStatus() (string, error)
}

// PbMux serves grpc-gateway requests for selected patterns using registered protobuf handlers.
type PbMux struct {
	Registrations []PbHandlerRegistration // Protobuf registrations to be registered in Mux.
	Patterns      []string                // URL patterns that will be handled by Mux.
	Mux           *gwruntime.ServeMux     // The mux that will be used for grpc-gateway requests.
}

// PbHandlerRegistration is a function that registers a protobuf handler.
type PbHandlerRegistration func(context.Context, *gwruntime.ServeMux, *grpc.ClientConn) error

// MuxHandler is a function that implements the mux handler functionality.
type MuxHandler func(http.Handler, http.ResponseWriter, *http.Request)

type Gateway struct {
	conn               *grpc.ClientConn
	pbHandlers         []PbMux
	muxHandler         MuxHandler
	chain              ChainAPI
	pool               AttestationAPI
	remoteAddr         string
	gatewayAddr        string
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.RWMutex
	maxCallRecvMsgSize uint64
	startFailure       error
	mux                *http.ServeMux
	server             *http.Server
	allowedOrigins     []string
}

func NewService(ctx context.Context, remoteAddr, gatewayAddr string, pbHandlers []PbMux, muxHandler MuxHandler, chain ChainAPI, pool AttestationAPI) *Gateway {
	g := &Gateway{
		ctx:            ctx,
		remoteAddr:     remoteAddr,
		gatewayAddr:    gatewayAddr,
		chain:          chain,
		pool:           pool,
		mu:             sync.RWMutex{},
		pbHandlers:     pbHandlers,
		muxHandler:     muxHandler,
		mux:            http.NewServeMux(),
		allowedOrigins: make([]string, 0),

		maxCallRecvMsgSize: 1 << 22,
	}

	return g
}

func (g *Gateway) WithAllowedOrigins(origins []string) *Gateway {
	g.allowedOrigins = origins
	return g
}

func (g *Gateway) corsMiddleware(h http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   g.allowedOrigins,
		AllowedMethods:   []string{http.MethodPost, http.MethodGet, http.MethodOptions},
		AllowCredentials: true,
		MaxAge:           600,
		AllowedHeaders:   []string{"*"},
	})
	return c.Handler(h)
}

func (g *Gateway) Start() {
	ctx, cancel := context.WithCancel(g.ctx)
	g.cancel = cancel

	conn, err := g.dial(ctx, g.remoteAddr)
	if err != nil {
		log.WithError(err).Error("Failed to connect to gRPC server")
		g.startFailure = err
		return
	}

	g.conn = conn

	for _, h := range g.pbHandlers {
		for _, r := range h.Registrations {
			if err := r(ctx, h.Mux, g.conn); err != nil {
				log.WithError(err).Error("Failed to register handler")
				g.startFailure = err
				return
			}
		}
		for _, p := range h.Patterns {
			g.mux.Handle(p, h.Mux)
		}
	}

	corsMux := g.corsMiddleware(g.mux)

	if g.muxHandler != nil {
		g.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			g.muxHandler(corsMux, w, r)
		})
	}

	g.server = &http.Server{
		Addr:    g.gatewayAddr,
		Handler: corsMux,
	}

	go func() {
		log.WithField("address", g.gatewayAddr).Info("Starting gRPC gateway")
		if err := g.server.ListenAndServe(); err != http.ErrServerClosed {
			log.WithError(err).Error("Failed to start gRPC gateway")
			g.startFailure = err
			return
		}
	}()
}

func (g *Gateway) Stop() error {
	if g.server != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(g.ctx, 2*time.Second)
		defer shutdownCancel()
		if err := g.server.Shutdown(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Warn("Existing connections terminated")
			} else {
				log.WithError(err).Error("Failed to gracefully shut down server")
			}
		}
	}

	if g.cancel != nil {
		g.cancel()
	}

	return nil
}

func (g *Gateway) Status() error {
	if g.startFailure != nil {
		return g.startFailure
	}

	if s := g.conn.GetState(); s != connectivity.Ready {
		return fmt.Errorf("grpc server is %s", s)
	}

	return nil
}

// dial the gRPC server.
func (g *Gateway) dial(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	security := grpc.WithInsecure()
	opts := []grpc.DialOption{
		security,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(g.maxCallRecvMsgSize))),
	}

	return grpc.DialContext(ctx, addr, opts...)
}
