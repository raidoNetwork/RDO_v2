package version

import (
	"fmt"
	"time"
)

// The value of these vars are set through linker options.
var ver = "0.6.4"
var buildDate = "{DATE}"

func Version() string {
	if buildDate == "{DATE}" {
		now := time.Now().Format(time.RFC3339)
		buildDate = now
	}

	data := fmt.Sprintf("Raido/%s", ver)

	return fmt.Sprintf("%s. Built at: %s", data, buildDate)
}
