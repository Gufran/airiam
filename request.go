package airiam

import "github.com/nlopes/slack"

// Request interface is used to pass messages
// to the intent handler.
type Request interface {
	// IntentName returns the name of the intent as
	// declared on intent registration.
	IntentName() string

	// SuggestedMessage returns the message returned
	// by Lex. Handler can return this same message
	// in response after handling the intent or customize it.
	SuggestedMessage() string

	// User returns the Slack user that is interacting
	// with the bot.
	User() *slack.User

	// Channel returns the Slack channel in which the
	// user is interacting with the bot.
	Channel() *slack.Channel
}

// IncomingMessage is a simple implementation of Request.
type IncomingMessage struct {
	intent       string
	suggestedMsg string
	user         *slack.User
	channel      *slack.Channel
}

func (i *IncomingMessage) IntentName() string {
	return i.intent
}

func (i *IncomingMessage) SuggestedMessage() string {
	return i.suggestedMsg
}

func (i *IncomingMessage) User() *slack.User {
	return i.user
}

func (i *IncomingMessage) Channel() *slack.Channel {
	return i.channel
}
