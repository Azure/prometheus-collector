package logger

import (
	"testing"

	"bytes"
	"encoding/json"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestEncodeEntry(t *testing.T) {
	testCases := []struct {
		name       string
		entry      zapcore.Entry
		fields     []zapcore.Field
		source     string
		wantOutput map[string]interface{}
	}{
		{
			name: "Test case 1",
			entry: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
			},
			fields:     []zapcore.Field{},
			source:     "test-source",
			wantOutput: map[string]interface{}{"source": "test-source", "msg": "test message", "level": "info"},
		},
		{
			name: "Test case 2",
			entry: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "another test message",
			},
			fields:     []zapcore.Field{},
			source:     "test-source-2",
			wantOutput: map[string]interface{}{"source": "test-source-2", "msg": "another test message", "level": "error"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ce := &customEncoder{
				Encoder: zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
				Source:  tc.source,
			}
			buf, _ := ce.EncodeEntry(tc.entry, tc.fields)
			var jsonOutput map[string]interface{}
			err := json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&jsonOutput)
			if err != nil {
				t.Fail()
			}
			for k, v := range tc.wantOutput {
				if jsonOutput[k] != v {
					t.Errorf("got %v, want %v", jsonOutput[k], v)
				}
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupLogger(&buf, "CPRemediatorLogs")
	assert.NotNil(t, logger)
	assert.IsType(t, &zap.Logger{}, logger)

	logger.Info("test message")
	curr := buf.String()
	// Collect log entry from setupLogger logger
	assert.Contains(t, "CPRemediatorLogs", curr)
	assert.Contains(t, "test message", curr)
	assert.Contains(t, "info", curr)
}
