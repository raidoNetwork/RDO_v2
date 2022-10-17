package metrics

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/raidoNetwork/RDO_v2/shared"
	"net/http"
	"runtime/debug"
	"runtime/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "prometheus")

var logCounterVec = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "log_entry_count",
	Help: "Log messages counter",
}, []string{"level", "prefix"})

type Service struct {
	server          *http.Server
	serviceRegistry *shared.ServiceRegistry
	failStatus      error
}

type rpcResponse struct {
	Error string `json:"error"`
	Data interface{} `json:"data"`
}

type serviceStatus struct {
	Name   string `json:"service"`
	Status bool   `json:"status"`
	Err    string `json:"error"`
}

func New(addr string, serviceRegistry *shared.ServiceRegistry) *Service {
	s := &Service{serviceRegistry: serviceRegistry}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		MaxRequestsInFlight: 5,
		Timeout:             30 * time.Second,
	}))
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/goroutines", s.goroutinesHandler)

	logrus.AddHook(s)

	s.server = &http.Server{Addr: addr, Handler: mux}

	return s
}

func (s *Service) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (s *Service) Fire(entry *logrus.Entry) error {
	prefix := "common"
	if prefixData, exists := entry.Data["prefix"]; exists {
		var ok bool
		prefix, ok = prefixData.(string)
		if !ok {
			return errors.New("bad prefix given")
		}
	}

	logCounterVec.WithLabelValues(entry.Level.String(), prefix).Inc()
	return nil
}

func (s *Service) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := rpcResponse{}

	var hasError bool
	var statuses []serviceStatus
	for stype, serviceErr := range s.serviceRegistry.Statuses() {
		s := serviceStatus{
			Name:   stype.String(),
			Status: true,
		}
		if serviceErr != nil {
			s.Status = false
			s.Err = serviceErr.Error()
			if s.Err != "" {
				hasError = true
			}
		}
		statuses = append(statuses, s)
	}
	response.Data = statuses

	if hasError {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error writing response: %s", err)
	}
}

func (_ *Service) goroutinesHandler(w http.ResponseWriter, _ *http.Request) {
	stack := debug.Stack()
	if _, err := w.Write(stack); err != nil {
		log.WithError(err).Error("Failed to write goroutines stack")
	}
	if err := pprof.Lookup("goroutine").WriteTo(w, 2); err != nil {
		log.WithError(err).Error("Failed to write pprof goroutines")
	}
}

// Start the prometheus service.
func (s *Service) Start() {
	go func() {
		log.WithField("address", s.server.Addr).Debug("Starting prometheus service")
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Errorf("Could not listen to host:port :%s: %v", s.server.Addr, err)
			s.failStatus = err
		}
	}()
}

// Stop the service gracefully.
func (s *Service) Stop() error {
	log.Info("Stop metrics server")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Status checks for any service failure conditions.
func (s *Service) Status() error {
	return s.failStatus
}
