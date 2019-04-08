package airiam

import (
	"context"
	"fmt"
	"github.com/nlopes/slack"
)

// Response represents the message to be sent to the user
type Response interface {
	GetMessage() string
}

// ReplyResponse is a simple response type with just
// one message.
type ReplyResponse struct {
	message string
}

func NewReplyResponse(r string, params ...interface{}) *ReplyResponse {
	return &ReplyResponse{
		message: fmt.Sprintf(r, params...),
	}
}

func (r *ReplyResponse) GetMessage() string {
	return r.message
}

// StreamResponse can be used to stream more than one
// line to the user.
type StreamResponse struct {
	message string
	stream  <-chan string
}

func NewStreamResponse(msg string, s <-chan string) *StreamResponse {
	return &StreamResponse{
		message: msg,
		stream:  s,
	}
}

func (r *StreamResponse) GetMessage() string {
	return r.message
}

type ErrorResponse struct {
	Message string
}

func NewErrorResponse(e error) *ErrorResponse {
	return &ErrorResponse{
		Message: e.Error(),
	}
}

func (r *ErrorResponse) Error() string {
	return r.GetMessage()
}

func (r *ErrorResponse) GetMessage() string {
	return fmt.Sprintf("**Error:** %s", r.Message)
}

type responseWriter struct {
	rtm   *slack.RTM
	ctx   context.Context
	event *slack.MessageEvent
}

func makeResponseWriter(ctx context.Context, rtm *slack.RTM, event *slack.MessageEvent) *responseWriter {
	return &responseWriter{
		rtm:   rtm,
		ctx:   ctx,
		event: event,
	}
}

func (r *responseWriter) write(m string) error {
	ts := r.event.ThreadTimestamp
	if ts == "" {
		ts = r.event.Timestamp
	}

	_, _, err := r.rtm.PostMessageContext(
		r.ctx,
		r.event.Channel,
		slack.MsgOptionTS(ts),
		slack.MsgOptionUser(r.rtm.GetInfo().User.ID),
		slack.MsgOptionAsUser(true),
		slack.MsgOptionText(m, false),
	)

	return err
}

func (r *responseWriter) stream(s <-chan string, log func(string)) {
	var (
		err error
		msg string
	)

	for {
		select {
		case <-r.ctx.Done():
			return

		case msg = <-s:
			err = r.write(msg)
			if err != nil {
				log(err.Error())
			}
		}
	}
}
