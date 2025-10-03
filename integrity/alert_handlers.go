package integrity

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogAlertHandler implements AlertHandler by logging alerts
type LogAlertHandler struct {
	logger *log.Logger
}

// NewLogAlertHandler creates a new log-based alert handler
func NewLogAlertHandler() *LogAlertHandler {
	return &LogAlertHandler{
		logger: log.New(os.Stdout, "[INTEGRITY-ALERT] ", log.LstdFlags),
	}
}

// HandleAlert handles an alert by logging it
func (lah *LogAlertHandler) HandleAlert(level AlertLevel, message string, details map[string]interface{}) error {
	logMessage := fmt.Sprintf("[%s] %s", level, message)

	if details != nil && len(details) > 0 {
		logMessage += " - Details: "
		for key, value := range details {
			logMessage += fmt.Sprintf("%s=%v ", key, value)
		}
	}

	lah.logger.Println(logMessage)
	return nil
}

// FileAlertHandler implements AlertHandler by writing alerts to a file
type FileAlertHandler struct {
	filePath string
	file     *os.File
}

// NewFileAlertHandler creates a new file-based alert handler
func NewFileAlertHandler(filePath string) (*FileAlertHandler, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open alert file: %w", err)
	}

	return &FileAlertHandler{
		filePath: filePath,
		file:     file,
	}, nil
}

// HandleAlert handles an alert by writing it to a file
func (fah *FileAlertHandler) HandleAlert(level AlertLevel, message string, details map[string]interface{}) error {
	timestamp := time.Now().Format(time.RFC3339)
	alertLine := fmt.Sprintf("%s [%s] %s", timestamp, level, message)

	if details != nil && len(details) > 0 {
		alertLine += " - Details: "
		for key, value := range details {
			alertLine += fmt.Sprintf("%s=%v ", key, value)
		}
	}

	alertLine += "\n"

	_, err := fah.file.WriteString(alertLine)
	if err != nil {
		return fmt.Errorf("failed to write alert to file: %w", err)
	}

	return fah.file.Sync()
}

// Close closes the file alert handler
func (fah *FileAlertHandler) Close() error {
	if fah.file != nil {
		return fah.file.Close()
	}
	return nil
}

// MultiAlertHandler implements AlertHandler by forwarding alerts to multiple handlers
type MultiAlertHandler struct {
	handlers []AlertHandler
}

// NewMultiAlertHandler creates a new multi-handler alert handler
func NewMultiAlertHandler(handlers ...AlertHandler) *MultiAlertHandler {
	return &MultiAlertHandler{
		handlers: handlers,
	}
}

// HandleAlert handles an alert by forwarding it to all registered handlers
func (mah *MultiAlertHandler) HandleAlert(level AlertLevel, message string, details map[string]interface{}) error {
	var errors []error

	for _, handler := range mah.handlers {
		if err := handler.HandleAlert(level, message, details); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple handler errors: %v", errors)
	}

	return nil
}

// AddHandler adds a new alert handler
func (mah *MultiAlertHandler) AddHandler(handler AlertHandler) {
	mah.handlers = append(mah.handlers, handler)
}

// ThresholdAlertHandler implements AlertHandler with level-based filtering
type ThresholdAlertHandler struct {
	minLevel AlertLevel
	handler  AlertHandler
}

// NewThresholdAlertHandler creates a new threshold-based alert handler
func NewThresholdAlertHandler(minLevel AlertLevel, handler AlertHandler) *ThresholdAlertHandler {
	return &ThresholdAlertHandler{
		minLevel: minLevel,
		handler:  handler,
	}
}

// HandleAlert handles an alert only if it meets the minimum level threshold
func (tah *ThresholdAlertHandler) HandleAlert(level AlertLevel, message string, details map[string]interface{}) error {
	if tah.shouldHandle(level) {
		return tah.handler.HandleAlert(level, message, details)
	}
	return nil
}

func (tah *ThresholdAlertHandler) shouldHandle(level AlertLevel) bool {
	levelPriority := map[AlertLevel]int{
		AlertLevelInfo:     1,
		AlertLevelWarning:  2,
		AlertLevelError:    3,
		AlertLevelCritical: 4,
	}

	return levelPriority[level] >= levelPriority[tah.minLevel]
}
