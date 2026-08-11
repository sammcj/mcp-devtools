package upstream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// Streamable HTTP lets a server answer a POST with either a JSON body or an
// event stream carrying the response as a message event, and the client has to
// accept whichever it gets. The official go-sdk server picks the event stream,
// so this path covers proxying any server built on it, including mcp-devtools
// itself.
//
// This parses an SSE-framed *response body*. It is unrelated to the SSE
// transport removed in 2026-07-28, which was a separate long-lived connection.

// maxEventStreamBytes bounds a response stream. A tool result is large but
// bounded; anything past this is an upstream that will not stop talking.
const maxEventStreamBytes = 64 << 20

// maxEventStreamLine bounds a single line, which bufio.Scanner would otherwise
// cap at 64KB and fail on a large tool result delivered as one data field.
const maxEventStreamLine = 8 << 20

// decodeEventStream returns the response to wantID from an event stream.
//
// A stream carries more than the response: comments, retry directives,
// keep-alive events, and any notification the upstream emits while the call
// runs, such as progress. Only the message answering wantID ends the read, so a
// notification arriving first is not mistaken for the response.
//
// Intervening notifications are dropped rather than forwarded. Relaying them to
// the downstream client would need a notification path the proxy does not have.
func decodeEventStream(body io.Reader, wantID any) (*Message, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, maxEventStreamBytes))
	scanner.Buffer(make([]byte, 0, 64<<10), maxEventStreamLine)

	// An event with no explicit type is a message event, per the SSE spec.
	eventType := ""
	var data strings.Builder

	dispatch := func() *Message {
		defer func() {
			eventType = ""
			data.Reset()
		}()

		if eventType != "" && eventType != "message" {
			return nil
		}
		payload := strings.TrimSuffix(data.String(), "\n")
		if strings.TrimSpace(payload) == "" {
			return nil
		}

		var msg Message
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			return nil
		}
		// A keep-alive or an unrelated payload decodes into an empty Message.
		// JSON-RPC 2.0 requires the version on every message, so it separates a
		// real response from anything else the stream carries.
		if msg.JSONRPC == "" {
			return nil
		}
		// A notification carries no id and answers nothing; a response to some
		// other request is not ours to consume either.
		if !sameID(msg.ID, wantID) {
			logrus.WithFields(logrus.Fields{
				"method": msg.Method,
				"id":     msg.ID,
			}).Debug("HTTP: skipping event stream message that is not the response")
			return nil
		}
		return &msg
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")

		// A blank line dispatches whatever has accumulated.
		if line == "" {
			if msg := dispatch(); msg != nil {
				return msg, nil
			}
			continue
		}

		// A line opening with a colon is a comment, commonly a keep-alive.
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			// A line with no colon is a field with an empty value, which is
			// meaningless for the fields handled here.
			continue
		}
		// Exactly one leading space after the colon is part of the framing.
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			eventType = value
		case "data":
			data.WriteString(value)
			data.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read event stream: %w", err)
	}

	// A stream can end without a trailing blank line.
	if msg := dispatch(); msg != nil {
		return msg, nil
	}

	return nil, fmt.Errorf("event stream ended without a response to request %v", wantID)
}

// sameID reports whether a response answers the request that was sent.
//
// Message.ID is `any` because JSON-RPC allows a string or a number, and a
// number always decodes back as float64 however it was sent. Comparing the
// canonical text of each keeps an id echoed as 1 matching the 1 that went out.
func sameID(got, want any) bool {
	if got == nil || want == nil {
		return false
	}
	return idString(got) == idString(want)
}

func idString(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

// isEventStream reports whether a Content-Type names an SSE body. The header
// carries parameters such as "; charset=utf-8", so this matches the media type
// rather than the whole value.
func isEventStream(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(mediaType), "text/event-stream")
}
