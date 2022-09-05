package flags

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

var (
	ValidatorKey = altsrc.NewStringFlag(&cli.StringFlag{
		Name:  "validator-key",
		Usage: "Path to validator key path",
	})
)
