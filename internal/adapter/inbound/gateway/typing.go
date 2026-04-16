package gateway

import "context"

// TypingNotifier is an optional interface that ChannelSender implementations
// can also implement to support typing indicators.
type TypingNotifier interface {
	SendTyping(ctx context.Context, channelID string) error
}
