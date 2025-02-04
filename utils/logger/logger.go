package logger

import (
	"github.com/sirupsen/logrus"
	"os"
)

var log = logrus.New()

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func Info(msg string) {
	log.Info(msg)
}

func Error(msg string) {
	log.Error(msg)
}

func Debug(msg string) {
	log.Debug(msg)
} 