package output

import "testing"

func TestLoggerMethods(t *testing.T) {
	logger := NewLogger("text", false, true)
	logger.Header("h")
	logger.Subheader("s")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")
	logger.Success("ok")
	logger.DryRun("dry")
	logger.Debug("dbg")
	logger.Table(map[string]string{"a": "b"})
}

func TestJSONLogger(t *testing.T) {
	logger := NewLogger("json", false, true)
	logger.Info("json")
	logger.Table(map[string]string{"x": "y"})
}
