package airiam

import (
	"context"
	"github.com/nlopes/slack"
)

// Intent defines an action that can be performed by the bot.
type Intent interface {
	// Authorize function can be used to perform checks
	// against the user that is interacting with the bot
	// and the channel they are interacting in.
	// This function is called as soon as the bot can determine
	// the intent a user is attempting to invoke.
	// The user is present with the error value returned by this
	// function if the value is not nil, otherwise the conversation
	// continues.
	Authorize(context.Context, *slack.User, *slack.Channel) error

	// GetParamsContainer returns a pointer to the struct that
	// can hold parameters required by the intent.
	// A non-pointer value will result in an error that will be
	// presented to the user.
	// To map values from one source attribute to a differently
	// named attribute in target struct you can use `attr` tag.
	GetParamsContainer() interface{}

	// PutParams fucntion takes arguments provided by user in form
	// of array of strings {intent, branch, application, environment}
	// and puts them into the assigned parameters
	PutParams(context.Context, []string) error

	Confirm(context.Context, []string) Response

	// Handle is responsible for handling the intent and
	// generating a response.
	Handle(context.Context, Request) Response
}
