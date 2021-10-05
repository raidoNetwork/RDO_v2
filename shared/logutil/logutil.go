package logutil

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"rdo_draft/shared/params"
)

func addLogWriter(w io.Writer) {
	mw := io.MultiWriter(logrus.StandardLogger().Out, w)
	logrus.SetOutput(mw)
}

// ConfigurePersistentLogging adds a log-to-file writer. File content is identical to stdout.
func ConfigurePersistentLogging(logFileName string) error {
	logrus.WithField("logFileName", logFileName).Info("Logs will be made persistent")
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, params.RaidoIoConfig().ReadWritePermissions) // #nosec G304
	if err != nil {
		return err
	}

	addLogWriter(f)

	logrus.Info("File logging initialized")
	return nil
}
