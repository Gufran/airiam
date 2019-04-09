package airiam

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lexruntimeservice"
	"github.com/aws/aws-sdk-go/service/lexruntimeservice/lexruntimeserviceiface"
	"github.com/mitchellh/mapstructure"
	"github.com/nlopes/slack"
	"strings"
	"sync"
)

// Engine maintains the event loop that enables the bot
// to send and receive messages.
type Engine struct {
	onConnectFn    func()
	onErrorFn      func(error)
	defaultEventFn func(ctx context.Context, event slack.RTMEvent)

	mutex   *sync.Mutex
	intents map[string]Intent

	selfId   string
	log      func(string)
	rtm      *slack.RTM
	lexName  string
	lexAlias string
	lex      lexruntimeserviceiface.LexRuntimeServiceAPI
}

// Config is required to create a new Engine.
type Config struct {
	// SlackToken is required and must be a valid bot token.
	// The bot will attempt to authenticate with Slack RTM API
	// on boot return an error on authentication failure.
	SlackToken string

	// IamRole is the ARN of AWS IAM role that should be
	// used by the bot to connect with Lex service.
	// If IamRole is empty the bot will use default AWS
	// credentials provider chain.
	IamRole string

	// AwsRegion is the name of AWS region where the Lex
	// bot is configured.
	AwsRegion string

	// LexBotName is the name of Lex bot that should
	// be used to parse incoming messages.
	LexBotName string

	// LexBotAlias is the version of lex bot to use.
	LexBotAlias string

	// LogFn will be used to log messages.
	LogFn func(string)

	// OnConnectFn is called when the bot successfully connects
	// with Slack RTM api.
	OnConnectFn func()

	// OnErrorFn is called when the RTM api returns an error.
	OnErrorFn func(error)

	// DefaultEventHandlerFn, if available, will be invoked
	// to handle RTM events that cannot be handled by the bot.
	DefaultEventHandlerFn func(context.Context, slack.RTMEvent)
}

// Create a new Engine instance.
func New(config *Config) (*Engine, error) {
	if config.SlackToken == "" {
		return nil, fmt.Errorf("SlackToken is not configured")
	}

	if config.AwsRegion == "" {
		r, err := getAwsRegion()
		if err != nil {
			return nil, fmt.Errorf("AWS region is not configured and failed to query current AWS region. %s", err.Error())
		}
		config.AwsRegion = r
	}

	lex, err := getLexClient(config.IamRole, config.AwsRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to build Lex runtime service client. %s", err)
	}

	client := slack.New(config.SlackToken)
	info, _, err := client.StartRTM()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize slack client. %s", err)
	}
	rtm := client.NewRTM()
	_, err = rtm.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with slack RTM service. %s", err)
	}

	return &Engine{
		onConnectFn:    config.OnConnectFn,
		onErrorFn:      config.OnErrorFn,
		defaultEventFn: config.DefaultEventHandlerFn,

		mutex:   &sync.Mutex{},
		intents: map[string]Intent{},

		selfId:   info.User.ID,
		log:      config.LogFn,
		rtm:      rtm,
		lexName:  config.LexBotName,
		lexAlias: config.LexBotAlias,
		lex:      lex,
	}, nil
}

// AddIntent is used to register a new intent or overwrite
// and already registered one.
func (e *Engine) AddIntent(name string, intent Intent) {
	e.mutex.Lock()
	e.intents[name] = intent
	e.mutex.Unlock()
}

// GetIntentOk returns an intent and true if the intent with
// given name is registered, otherwise it returns nil and false.
func (e *Engine) GetIntentOk(intent string) (Intent, bool) {
	e.mutex.Lock()
	i, ok := e.intents[intent]
	e.mutex.Unlock()
	return i, ok
}

// Starts the event loop
func (e *Engine) Boot(ctx context.Context) error {
	go e.rtm.ManageConnection()

	for {
		select {
		case <-ctx.Done():
			err := e.rtm.Disconnect()
			return fmt.Errorf("%s; %s", ctx.Err(), err)

		case msg, ok := <-e.rtm.IncomingEvents:
			if !ok {
				return nil
			}

			switch event := msg.Data.(type) {
			case *slack.ConnectedEvent:
				if e.onConnectFn != nil {
					go e.onConnectFn()
				}

			case *slack.MessageEvent:
				info := e.rtm.GetInfo()
				if len(event.User) == 0 || event.User == "USLACKBOT" || event.User == info.User.ID || len(event.BotID) > 0 {
					continue
				}

				mentionMe := fmt.Sprintf("<@%s>", info.User.ID)
				if strings.Contains(event.Text, mentionMe) {
					go e.handleMessage(ctx, event)
				}

			case *slack.RTMError:
				if e.onErrorFn != nil {
					go e.onErrorFn(event)
				}

			case *slack.InvalidAuthEvent:
				return fmt.Errorf("authentication error")

			default:
				if e.defaultEventFn != nil {
					go e.defaultEventFn(ctx, msg)
				}
			}
		}
	}
}

func (e *Engine) selfMention() string {
	return fmt.Sprintf("<@%s>", e.selfId)
}

func (e *Engine) handleMessage(ctx context.Context, msg *slack.MessageEvent) {
	respWriter := makeResponseWriter(ctx, e.rtm, msg)
	resp := e.brokerMessage(ctx, msg)
	err := respWriter.write(resp.GetMessage())
	if err != nil {
		e.log(err.Error())
	}

	switch sr := resp.(type) {
	case *StreamResponse:
		go respWriter.stream(sr.stream, e.log)
	}
}

func (e *Engine) brokerMessage(ctx context.Context, msg *slack.MessageEvent) Response {
	msgText := strings.Replace(msg.Text, e.selfMention(), "", -1)
	ts := msg.ThreadTimestamp
	if ts == "" {
		ts = msg.Timestamp
	}

	userId := fmt.Sprintf("%s-%s-%s", msg.Channel, msg.User, ts)

	input := &lexruntimeservice.PostTextInput{
		BotAlias:  aws.String(e.lexAlias),
		BotName:   aws.String(e.lexName),
		InputText: aws.String(msgText),
		UserId:    aws.String(userId),
	}

	output, err := e.lex.PostText(input)
	if err != nil {
		return NewErrorResponse(err)
	}

	intent := aws.StringValue(output.IntentName)
	state := aws.StringValue(output.DialogState)
	msgUser, err := e.rtm.GetUserInfo(msg.User)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to load user details for user ID %q. %s", msg.User, err))
	}

	msgChannel, err := e.rtm.GetConversationInfoContext(ctx, msg.Channel, false)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to load channel details for channel ID %q. %s", msg.Channel, err))
	}

	if state == "ElicitIntent" || state == "Failed" {
		return NewReplyResponse(aws.StringValue(output.Message))
	}

	handler, ok := e.GetIntentOk(intent)
	if !ok {
		return NewErrorResponse(fmt.Errorf("I understand your request but I don't know how to handle it yet. Intent: %q", intent))
	}

	err = handler.Authorize(ctx, msgUser, msgChannel)
	if err != nil {
		return NewErrorResponse(err)
	}

	switch state {
	case "ConfirmIntent", "ElicitSlot":
		return NewReplyResponse(aws.StringValue(output.Message))
	case "ReadyForFulfillment":
		return NewReplyResponse("Lex says that the intent is ready for fulfillment, I cannot handle this dialog state yet.")
	case "Fulfilled":
		err := loadParams(handler.GetParamsContainer(), output.Slots)
		if err != nil {
			return NewErrorResponse(err)
		}

		return handler.Handle(ctx, &IncomingMessage{
			intent:       *output.IntentName,
			suggestedMsg: *output.Message,
			user:         msgUser,
			channel:      msgChannel,
		})
	default:
		// Unknown dialog state.
		// This can happen due to a change in Lex API response
		// but otherwise we should never hit this case.
		return NewReplyResponse("I'm having problem with AWS Lex service response. Lex returned the dialog state %q and I don't know what to do with that. Perhaps you can update my code to be compatible with latest Lex API", state)
	}
}

func loadParams(container interface{}, attrs map[string]*string) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused:      true,
		ZeroFields:       true,
		WeaklyTypedInput: true,
		Result:           container,
		TagName:          "attr",
	})

	if err != nil {
		return fmt.Errorf("failed to initialize attribute decoder. %s", err)
	}

	return decoder.Decode(attrs)
}
