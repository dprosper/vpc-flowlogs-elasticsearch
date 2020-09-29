/*
Copyright © 2020 Dimitri Prosper <dimitri_prosper@us.ibm.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package logger

import (
	"path"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SystemLogger variable
var SystemLogger *zap.Logger

// ErrorLogger variable
var ErrorLogger *zap.Logger

// InitLogger function
func InitLogger() {
	systemLogWriter := getLogWriter("system.log")
	systemCore := zapcore.NewCore(getFileEncoder(), systemLogWriter, zapcore.DebugLevel)
	SystemLogger = zap.New(systemCore, zap.AddCaller())
	defer SystemLogger.Sync()

	errorLogWriter := getLogWriter("error.log")
	errorCore := zapcore.NewCore(getFileEncoder(), errorLogWriter, zapcore.DebugLevel)
	ErrorLogger = zap.New(errorCore, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	defer ErrorLogger.Sync()
}

func getFileEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.MessageKey = "message"
	encoderConfig.TimeKey = "timestamp"
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(filename string) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   path.Join("./logs", filename),
		MaxSize:    5,
		MaxBackups: 500,
		MaxAge:     14,
		Compress:   true,
		LocalTime:  true,
	}

	lumberJackLogger.Rotate()

	return zapcore.AddSync(lumberJackLogger)
}
