package gateway

import (
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"google.golang.org/protobuf/encoding/protojson"
)

// MuxConfig contains configuration that should be used when registering the beacon node in the gateway.
type MuxConfig struct {
	Handler MuxHandler
	PbMux   PbMux
}

// DefaultConfig returns a fully configured MuxConfig with standard gateway behavior.
func DefaultConfig() MuxConfig {
	v1Registrations := []PbHandlerRegistration{
		prototype.RegisterRaidoChainServiceHandler,
		prototype.RegisterAttestationServiceHandler,
	}

	v1Mux := gwruntime.NewServeMux(
		gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, &gwruntime.HTTPBodyMarshaler{
			Marshaler: &gwruntime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:   true,
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			},
		}),
	)

	v1PbHandler := PbMux{
		Registrations: v1Registrations,
		Patterns:      []string{"/rdo/v1/"},
		Mux:           v1Mux,
	}

	return MuxConfig{
		PbMux: v1PbHandler,
	}
}
