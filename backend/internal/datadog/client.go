package datadog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"lastsaas/internal/apicounter"
	"lastsaas/internal/models"
)

const (
	metricsBufferSize = 100
	eventsBufferSize  = 50
	flushInterval     = 10 * time.Second
	maxBackoff        = 60 * time.Second
	httpTimeout       = 10 * time.Second
)

// Client is an async-buffered DataDog REST API client.
// It submits custom metrics (from telemetry events) and events (from syslog)
// without requiring a DataDog Agent.
type Client struct {
	apiKey     string
	site       string // e.g. "us5.datadoghq.com"
	env        string // e.g. "dev", "prod"
	appName    string
	httpClient *http.Client

	metricsCh chan metricPoint
	eventsCh  chan ddEvent
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// metricPoint is a single count increment to be batched.
type metricPoint struct {
	MetricName string
	Tags       []string
	Value      float64
	Timestamp  int64 // Unix seconds
}

// ddEvent is a DataDog event (from syslog).
type ddEvent struct {
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	Priority  string   `json:"priority"`
	AlertType string   `json:"alert_type"`
	Tags      []string `json:"tags"`
}

// New creates a DataDog client and starts background flush goroutines.
func New(apiKey, site, env, appName string) *Client {
	c := &Client{
		apiKey:     apiKey,
		site:       site,
		env:        env,
		appName:    appName,
		httpClient: &http.Client{Timeout: httpTimeout},
		metricsCh:  make(chan metricPoint, metricsBufferSize),
		eventsCh:   make(chan ddEvent, eventsBufferSize),
		stopCh:     make(chan struct{}),
	}
	c.wg.Add(2)
	go c.metricsFlushLoop()
	go c.eventsFlushLoop()
	return c
}

// Stop gracefully drains buffers and shuts down flush loops.
func (c *Client) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// TrackTelemetryEvent converts a TelemetryEvent into a count metric and enqueues it.
func (c *Client) TrackTelemetryEvent(event models.TelemetryEvent) {
	point := metricPoint{
		MetricName: "lastsaas.telemetry.event",
		Tags: []string{
			"event_name:" + event.EventName,
			"category:" + event.Category,
			"env:" + c.env,
			"app:" + c.appName,
		},
		Value:     1,
		Timestamp: event.CreatedAt.Unix(),
	}
	select {
	case c.metricsCh <- point:
	default:
		slog.Warn("datadog: metrics buffer full, dropping telemetry point", "event", event.EventName)
	}
}

// TrackSyslogEntry converts a critical/high SystemLog into a DataDog event.
func (c *Client) TrackSyslogEntry(entry models.SystemLog) {
	alertType := "warning"
	if entry.Severity == models.LogCritical {
		alertType = "error"
	}

	tags := []string{
		"severity:" + string(entry.Severity),
		"env:" + c.env,
		"app:" + c.appName,
	}
	if entry.Category != "" {
		tags = append(tags, "category:"+string(entry.Category))
	}

	evt := ddEvent{
		Title:     fmt.Sprintf("[%s] %s", entry.Severity, truncate(entry.Message, 100)),
		Text:      entry.Message,
		Priority:  "normal",
		AlertType: alertType,
		Tags:      tags,
	}
	select {
	case c.eventsCh <- evt:
	default:
		slog.Warn("datadog: events buffer full, dropping syslog entry", "severity", entry.Severity)
	}
}

// Validate checks whether the DataDog API key is valid.
func (c *Client) Validate(ctx context.Context) error {
	url := fmt.Sprintf("https://api.%s/api/v1/validate", c.site)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("DD-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("datadog validate request failed: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	apicounter.DataDogAPICalls.Add(1)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("datadog API key validation failed: status %d", resp.StatusCode)
	}
	return nil
}

// metricsFlushLoop batches metric points and submits them to DataDog.
func (c *Client) metricsFlushLoop() {
	defer c.wg.Done()
	backoff := flushInterval
	timer := time.NewTimer(backoff)
	timer.Stop()

	buf := make([]metricPoint, 0, metricsBufferSize)
	flush := func() bool {
		if len(buf) == 0 {
			return true
		}
		if err := c.submitMetrics(buf); err != nil {
			slog.Warn("datadog: metrics flush failed, will retry", "count", len(buf), "error", err)
			return false
		}
		buf = buf[:0]
		return true
	}

	for {
		select {
		case pt := <-c.metricsCh:
			wasEmpty := len(buf) == 0
			buf = append(buf, pt)
			if len(buf) >= metricsBufferSize {
				if flush() {
					backoff = flushInterval
				} else {
					backoff = min(backoff*2, maxBackoff)
				}
			}
			if wasEmpty && len(buf) > 0 {
				timer.Reset(backoff)
			}
		case <-timer.C:
			if flush() {
				backoff = flushInterval
			} else {
				backoff = min(backoff*2, maxBackoff)
			}
			if len(buf) > 0 {
				timer.Reset(backoff)
			}
		case <-c.stopCh:
			timer.Stop()
			for {
				select {
				case pt := <-c.metricsCh:
					buf = append(buf, pt)
				default:
					flush()
					return
				}
			}
		}
	}
}

// eventsFlushLoop sends syslog events to DataDog one at a time.
func (c *Client) eventsFlushLoop() {
	defer c.wg.Done()
	for {
		select {
		case evt := <-c.eventsCh:
			if err := c.submitEvent(evt); err != nil {
				slog.Warn("datadog: event submission failed", "title", evt.Title, "error", err)
			}
		case <-c.stopCh:
			// Drain remaining events
			for {
				select {
				case evt := <-c.eventsCh:
					if err := c.submitEvent(evt); err != nil {
						slog.Warn("datadog: event submission failed during shutdown", "error", err)
					}
				default:
					return
				}
			}
		}
	}
}

// submitMetrics sends batched metrics via POST /api/v2/series.
func (c *Client) submitMetrics(points []metricPoint) error {
	// Group points by metric+tags combination for proper series structure.
	type seriesKey struct {
		metric string
		tags   string
	}
	groups := make(map[seriesKey][]metricPoint)
	for _, p := range points {
		key := seriesKey{metric: p.MetricName, tags: fmt.Sprint(p.Tags)}
		groups[key] = append(groups[key], p)
	}

	type ddPoint struct {
		Timestamp int64   `json:"timestamp"`
		Value     float64 `json:"value"`
	}
	type ddSeries struct {
		Metric string     `json:"metric"`
		Type   int        `json:"type"`
		Points []ddPoint  `json:"points"`
		Tags   []string   `json:"tags"`
	}

	series := make([]ddSeries, 0, len(groups))
	for _, pts := range groups {
		ddPts := make([]ddPoint, len(pts))
		for i, p := range pts {
			ddPts[i] = ddPoint{Timestamp: p.Timestamp, Value: p.Value}
		}
		series = append(series, ddSeries{
			Metric: pts[0].MetricName,
			Type:   1, // count
			Points: ddPts,
			Tags:   pts[0].Tags,
		})
	}

	payload := struct {
		Series []ddSeries `json:"series"`
	}{Series: series}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.%s/api/v2/series", c.site)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	apicounter.DataDogAPICalls.Add(1)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("datadog metrics API returned status %d", resp.StatusCode)
	}
	return nil
}

// submitEvent sends a single event via POST /api/v1/events.
func (c *Client) submitEvent(evt ddEvent) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.%s/api/v1/events", c.site)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	apicounter.DataDogAPICalls.Add(1)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("datadog events API returned status %d", resp.StatusCode)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
