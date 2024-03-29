package slot

import (
	"net"
	"sort"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/raidoNetwork/RDO_v2/shared/params"
)

// durationSlice attaches the methods of sort.Interface to []time.Duration,
// sorting in increasing order.
type durationSlice []time.Duration

func (s durationSlice) Len() int           { return len(s) }
func (s durationSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s durationSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// checkClockDrift queries an NTP server for clock drifts and warns the user if
// one large enough is detected.
func checkClockDrift() {
	config := params.MainnetConfig()
	drift, err := sntpDrift(config.NTPChecks)
	if err != nil {
		log.Warnf("Error occured when measuring drift: %s", err)
		return
	}

	driftThreshold := time.Millisecond * time.Duration(config.NTPThreshold)

	if drift < -driftThreshold || drift > driftThreshold {
		log.Warnf("System clock seems off by %v, which can prevent network connectivity", drift)
		log.Warn("Please synchronize with the network's time.")
	} else {
		log.Debugf("NTP sanity check done, drift: %v", drift)
	}
}

// sntpDrift does a naive time resolution against an NTP server and returns the
// measured drift. This method uses the simple version of NTP. It's not precise
// but should be fine for these purposes.
//
// Note, it executes two extra measurements compared to the number of requested
// ones to be able to discard the two extremes as outliers.
func sntpDrift(measurements int) (time.Duration, error) {
	config := params.MainnetConfig()
	// Resolve the address of the NTP server
	addr, err := net.ResolveUDPAddr("udp", config.NTPPool+":123")
	if err != nil {
		return 0, errors.Wrap(err, "Error when resolving NTP server address")
	}
	// Construct the time request (empty package with only 2 fields set):
	//   Bits 3-5: Protocol version, 3
	//   Bits 6-8: Mode of operation, client, 3
	request := make([]byte, 48)
	request[0] = 3<<3 | 3

	// Execute each of the measurements
	drifts := []time.Duration{}
	for i := 0; i < measurements+2; i++ {
		// Dial the NTP server and send the time retrieval request
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return 0, errors.Wrap(err, "Error when dialing and requesting time retrieval")
		}
		defer conn.Close()

		sent := time.Now()
		if _, err = conn.Write(request); err != nil {
			return 0, errors.Wrap(err, "Error when writing")
		}
		// Retrieve the reply and calculate the elapsed time
		conn.SetDeadline(time.Now().Add(7 * time.Second))

		reply := make([]byte, 48)
		if _, err = conn.Read(reply); err != nil {
			return 0, errors.Wrap(err, "Error when reading")
		}
		elapsed := time.Since(sent)

		// Reconstruct the time from the reply data
		sec := uint64(reply[43]) | uint64(reply[42])<<8 | uint64(reply[41])<<16 | uint64(reply[40])<<24
		frac := uint64(reply[47]) | uint64(reply[46])<<8 | uint64(reply[45])<<16 | uint64(reply[44])<<24

		nanosec := sec*1e9 + (frac*1e9)>>32

		t := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(nanosec)).Local()

		// Calculate the drift based on an assumed answer time of RRT/2
		drifts = append(drifts, sent.Sub(t)+elapsed/2)
	}
	// Calculate average drift (drop two extremities to avoid outliers)
	sort.Sort(durationSlice(drifts))

	drift := time.Duration(0)
	for i := 1; i < len(drifts)-1; i++ {
		drift += drifts[i]
	}
	return drift / time.Duration(measurements), nil
}
