package utils

import (
	"bufio"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"os"
	"pvc-operator/constants"
)

var Logger *logrus.Logger

func init() {
	Logger = logrus.New()
	Logger.SetLevel(logrus.DebugLevel)
	pathMap := lfshook.PathMap{
		logrus.InfoLevel:  constants.LOGGER_ROOT + "info.log",
		logrus.ErrorLevel: constants.LOGGER_ROOT + "error.log",
		logrus.DebugLevel: constants.LOGGER_ROOT + "debug.log",
	}
	hook := lfshook.NewHook(pathMap, &logrus.JSONFormatter{})
	Logger.AddHook(hook)
	Logger.SetReportCaller(true)
}

func GetLogger(subject string) *logrus.Logger {
	logger := logrus.New()
	src, err := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		panic(err)
	}
	logger.Out = bufio.NewWriter(src)
	logger.SetLevel(logrus.DebugLevel)
	pathMap := lfshook.PathMap{
		logrus.InfoLevel:  constants.LOGGER_ROOT + subject + ".log",
		logrus.ErrorLevel: constants.LOGGER_ROOT + subject + ".log",
		logrus.DebugLevel: constants.LOGGER_ROOT + subject + ".log",
	}
	hook := lfshook.NewHook(pathMap, &logrus.JSONFormatter{})
	logger.AddHook(hook)

	return logger
}

func Info(v ...interface{}) {
	//prod模式不输出Info日志
	if constants.RUN_MODE != constants.RUN_MODE_PROD {
		Logger.Info(v...)
	}
}

func Debug(v ...interface{}) {
	//只要dev模式才输出Debug日志
	if constants.RUN_MODE == constants.RUN_MODE_DEV {
		Logger.Debug(v...)
	}
}

func Error(v ...interface{}) {
	Logger.Error(v...)
}

func Warn(v ...interface{}) {
	Logger.Warn(v...)
}

func Subject(subject string, v ...interface{}) {
	Logger.WithFields(logrus.Fields{
		"subject": subject,
	}).Info(v...)
}
