package version

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// The value of these vars are set through linker options.
var gitCommit = "0.0.1"
var buildDate = "{DATE}"
var buildDateUnix = "0"
var gitTag = "latest"

func Version() string {
	if buildDate == "{DATE}" {
		now := time.Now().Format(time.RFC3339)
		buildDate = now
	}
	if buildDateUnix == "{DATE_UNIX}" {
		buildDateUnix = strconv.Itoa(int(time.Now().Unix()))
	}
	return fmt.Sprintf("%s. Built at: %s", BuildData(), buildDate)
}

// BuildData returns the git tag and commit of the current build.
func BuildData() string {
	// if doing a local build, these values are not interpolated
	if gitCommit == "{STABLE_GIT_COMMIT}" {
		commit, err := exec.Command("git", "rev-parse", "HEAD").Output()
		if err != nil {
			log.Println(err)
		} else {
			gitCommit = strings.TrimRight(string(commit), "\r\n")
		}
	}
	return fmt.Sprintf("Raido/%s/%s", gitTag, gitCommit)
}
