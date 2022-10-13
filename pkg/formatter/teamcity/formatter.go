package teamcity

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// https://www.jetbrains.com/help/teamcity/service-messages.html

type TeamcityFormatter struct{}

const (
	StatusFailure           = "FAILURE"
	StatusError             = "ERROR"
	StatusWarning           = "WARNING"
	StatusNormal            = "NORMAL"
	MessageName             = "messageName"
	MessageNameBuildProblem = "buildProblem"
	MessageNameBuildStatus  = "buildStatus"
	MessageNameBlockOpened  = "blockOpened"
	MessageNameBlockClosed  = "blockClosed"
)

var escapeChars = map[string]string{
	"|":  "||",
	"'":  "|'",
	"\n": "|n",
	"\r": "|r",
	"[":  "|[",
	"]":  "|]",
}

func (f *TeamcityFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	output := []string{}

	message, ok := entry.Data[MessageName]
	if !ok {
		message = "message"
	}

	output = append(output, escape(message.(string)))

	for k, v := range entry.Data {
		if k == MessageName {
			continue
		}
		output = append(output, fmt.Sprintf("%s='%s'", k, escape(v.(string))))
	}

	if _, ok := entry.Data["status"]; !ok {
		output = append(output, fmt.Sprintf("status='%s'", getStatus(entry.Level.String())))
	}

	if message == MessageNameBuildProblem || message == MessageNameBlockOpened {
		output = append(output, fmt.Sprintf("description='%s'", escape(entry.Message)))
	} else {
		output = append(output, fmt.Sprintf("text='%s'", escape(entry.Message)))

	}

	return []byte(fmt.Sprintf("##teamcity[%s]\n", strings.Join(output, " "))), nil
}

func escape(text string) string {
	output := text
	for origin, escaped := range escapeChars {
		output = strings.ReplaceAll(output, origin, escaped)
	}
	return output
}

func getStatus(level string) string {
	switch strings.ToLower(level) {
	case "panic", "fatal":
		return StatusFailure
	case "error":
		return StatusError
	case "warn", "warning":
		return StatusWarning
	case "info", "trace", "debug":
		return StatusNormal
	}
	return StatusNormal
}
