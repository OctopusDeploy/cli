package wait

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
)

const (
	indentSize       = 4
	separator        = ""
	sepLength        = 29
	timeFormat       = "02-01-2006 15:04:05"
	logIndentLevel   = 6
	taskHeaderIndent = ""
	logLineIndent    = "                  "
)

type TaskOutputFormatter struct {
	out io.Writer
}

func NewTaskOutputFormatter(out io.Writer) *TaskOutputFormatter {
	return &TaskOutputFormatter{
		out: out,
	}
}

func (f *TaskOutputFormatter) PrintTaskInfo(t *tasks.Task) {
	status := f.formatTaskStatus(t.State)
	if t.StartTime != nil && t.CompletedTime != nil {
		duration := t.CompletedTime.Sub(*t.StartTime).Round(time.Second)
		timeInfo := f.formatTaskHeader(t.ID, t.Description, status, t.StartTime, t.CompletedTime, duration)
		fmt.Fprintln(f.out, timeInfo)
	} else {
		fmt.Fprintln(f.out, f.formatTaskHeader(t.ID, t.Description, status, nil, nil, time.Duration(0)))
	}
}

func (f *TaskOutputFormatter) PrintActivityElement(activity *tasks.ActivityElement, indent int, completedChildIds map[string]bool) {
	for _, child := range activity.Children {
		if child.Status != "Pending" && child.Status != "Running" && !completedChildIds[child.ID] {
			line := fmt.Sprintf("         %s: %s", child.Status, child.Name)

			var timeInfo string
			if child.Started != nil && child.Ended != nil {
				startTime := child.Started.Format(timeFormat)
				endTime := child.Ended.Format(timeFormat)
				duration := child.Ended.Sub(*child.Started).Round(time.Second)
				indentStr := f.getIndentation(logIndentLevel)
				sep := f.formatSeparatorLine(indentStr)
				timeInfo = fmt.Sprintf("\n%s\n%sStarted:   %s\n%sEnded:     %s\n%sDuration:  %s\n%s",
					sep,
					indentStr, startTime,
					indentStr, endTime,
					indentStr, duration,
					sep)
			}

			switch child.Status {
			case "Success":
				line = output.Green(line)
			case "Failed":
				line = output.Red(line)
			case "Skipped":
				line = output.Yellow(line)
			case "SuccessWithWarning":
				line = output.Yellow(line)
			case "Canceled":
				line = output.Yellow(line)
			}

			if timeInfo != "" {
				line = line + timeInfo
			}
			fmt.Fprintln(f.out, line)

			for _, stepChild := range child.Children {
				if stepChild.Status != "Pending" && stepChild.Status != "Running" {
					var lastWasRetry bool
					for _, logElement := range stepChild.LogElements {
						message := logElement.MessageText
						timeStr := logElement.OccurredAt.Format(timeFormat)
						category := logElement.Category

						if strings.Contains(message, "Retry (attempt") {
							fmt.Fprintln(f.out, f.formatRetryMessage(message))
							lastWasRetry = true
						} else if lastWasRetry && strings.Contains(message, "Starting") {
							lastWasRetry = false
						}

						logLine := f.formatLogLine(timeStr, category, message)
						switch strings.ToLower(category) {
						case "warning":
							logLine = output.Yellow(logLine)
						case "error", "fatal":
							logLine = output.Red(logLine)
						}

						fmt.Fprintln(f.out, logLine)
					}
				}
			}

			completedChildIds[child.ID] = true
		}
	}
}

func (f *TaskOutputFormatter) formatTaskStatus(state string) string {
	switch state {
	case "Failed", "TimedOut":
		return output.Red(state)
	case "Success":
		return output.Green(state)
	case "Queued", "Executing", "Cancelling", "Canceled":
		return output.Yellow(state)
	default:
		return state
	}
}

func (f *TaskOutputFormatter) formatTaskHeader(taskID string, description string, status string, startTime *time.Time, endTime *time.Time, duration time.Duration) string {
	if startTime == nil || endTime == nil {
		return fmt.Sprintf("%s: %s: %s", taskID, description, status)
	}

	return fmt.Sprintf("\n%s %s %s\n   Name: %s\n   Status: %s\n   Started: %s\n   Ended: %s\n   Duration: %s\n",
		taskHeaderIndent,
		taskID,
		taskHeaderIndent,
		description,
		status,
		startTime.Format(timeFormat),
		endTime.Format(timeFormat),
		duration)
}

func (f *TaskOutputFormatter) formatLogLine(timeStr, category, message string) string {
	return fmt.Sprintf("%s%-19s      %-8s %s", logLineIndent, timeStr, category, message)
}

func (f *TaskOutputFormatter) formatRetryMessage(message string) string {
	return fmt.Sprintf("%s%s", logLineIndent, output.Yellow(fmt.Sprintf("------ %s ------", message)))
}

func (f *TaskOutputFormatter) getIndentation(level int) string {
	return strings.Repeat(" ", level*indentSize)
}

func (f *TaskOutputFormatter) formatSeparatorLine(indent string) string {
	return indent + strings.Repeat(separator, sepLength)
}
