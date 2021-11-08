package cmd

import (
	"path/filepath"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"runtime"
)

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := fileutil.HomeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "RDO")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Local", "RDO")
		} else {
			return filepath.Join(home, ".rdo")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}
