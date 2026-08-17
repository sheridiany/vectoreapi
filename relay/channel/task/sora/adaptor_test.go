package sora

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test12AIVideoRequestUsesUnifiedTaskEndpointAndInput(t *testing.T) {
	body := map[string]interface{}{
		"model":    "seedance-2.0-fast",
		"prompt":   "test",
		"duration": 8,
		"size":     "1280x720",
	}

	adapt12AIVideoRequest(body, "seedance-2.0-fast")

	assert.Equal(t, "seedance-2.0-fast", body["model"])
	assert.NotContains(t, body, "prompt")
	assert.Equal(t, "test", body["input"].(map[string]interface{})["prompt"])
	assert.Equal(t, 8, body["input"].(map[string]interface{})["duration"])
	assert.Equal(t, "720p", body["input"].(map[string]interface{})["resolution"])
	assert.Equal(t, "16:9", body["input"].(map[string]interface{})["aspect_ratio"])
}

func Test12AIVideoRequestMapsPortraitSizeToAspectRatio(t *testing.T) {
	body := map[string]interface{}{
		"prompt": "test",
		"size":   "720x1280",
	}

	adapt12AIVideoRequest(body, "seedance-2.5")

	assert.Equal(t, "9:16", body["input"].(map[string]interface{})["aspect_ratio"])
}

func Test12AIProgressAcceptsObjectPayload(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"status":"processing","progress":{"percent":42}}`))

	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, result.Status)
	assert.Equal(t, "42%", result.Progress)
}

func TestTaskSubmitReqReads12AIInput(t *testing.T) {
	var req relaycommon.TaskSubmitReq
	err := json.Unmarshal([]byte(`{"model":"MiniMax-H3","input":{"prompt":"test","duration":10,"resolution":"2K","image_references":["https://example.com/a.jpg"]}}`), &req)

	require.NoError(t, err)
	assert.Equal(t, "test", req.Prompt)
	assert.Equal(t, 10, req.Duration)
	assert.Equal(t, "2K", req.Resolution)
	assert.True(t, req.HasImage())
}

func TestSoraBuildRequestBodyReturnsReplayablePassThroughBody(t *testing.T) {
	payload := []byte("opaque-sora-request-body")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/octet-stream")
	defer common.CleanupBodyStorage(c)

	info := &relaycommon.RelayInfo{}
	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	replayable, ok := body.(common.ReplayableBody)
	require.True(t, ok)

	sent, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, payload, sent)
	assert.EqualValues(t, len(payload), replayable.Size())

	replayBody, err := replayable.NewReader()
	require.NoError(t, err)
	replay, err := io.ReadAll(replayBody)
	require.NoError(t, err)
	require.NoError(t, replayBody.Close())
	assert.Equal(t, payload, replay)
}
