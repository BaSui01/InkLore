package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

const sseWriteDeadline = 10 * time.Second

func (h *Handler) subscribeEvents(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Events == nil {
		response.Fail(c, bad())
		return
	}

	runID := c.Param("run_id")

	if c.Request.URL.RawQuery != "" {
		response.Fail(c, bad())
		return
	}
	if err := httpguard.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	lastEventID := c.GetHeader("Last-Event-ID")
	lastEventID = strings.TrimSpace(lastEventID)

	underlying, ok := c.Writer.(interface{ Unwrap() http.ResponseWriter })
	if !ok {
		response.Fail(c, bad())
		return
	}
	// Gin Flush discards errors; control the underlying writer while recording writes through Gin.
	controller := http.NewResponseController(underlying.Unwrap())
	streamStarted := false
	writeFrame := func(frame []byte) error {
		if err := controller.SetWriteDeadline(time.Now().Add(sseWriteDeadline)); err != nil {
			return err
		}
		defer func() { _ = controller.SetWriteDeadline(time.Time{}) }()

		if !streamStarted {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache, no-transform")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
		}
		written, err := c.Writer.Write(frame)
		// Writing may have committed headers even when the peer has disconnected.
		streamStarted = true
		if err != nil {
			return err
		}
		if written != len(frame) {
			return io.ErrShortWrite
		}
		return controller.Flush()
	}

	err := h.Events.SubscribeEvents(
		c.Request.Context(),
		p,
		runID,
		lastEventID,
		func(envelope d.SSEEnvelope, eventName string) error {
			dataBytes, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			frame := []byte(fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", envelope.Seq, eventName, dataBytes))
			return writeFrame(frame)
		},
		func() error {
			return writeFrame([]byte(": heartbeat\n\n"))
		},
	)

	if err != nil && !streamStarted {
		// Preflight error occurred before stream was established (e.g. 410 cursor expired, 400 cursor invalid)
		response.Fail(c, err)
		return
	}
}
