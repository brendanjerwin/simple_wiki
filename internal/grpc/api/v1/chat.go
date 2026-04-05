package v1

import (
	"context"
	"errors"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const errPageRequired = "page is required"

// SendMessage implements the SendMessage RPC.
// Receives a user message, assigns a UUID, writes it to the buffer, and pushes it to channel subscribers.
func (s *Server) SendMessage(ctx context.Context, req *apiv1.SendChatMessageRequest) (*apiv1.SendChatMessageResponse, error) {
	if req.Page == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}

	// Get sender name from Tailscale identity
	identity := tailscale.IdentityFromContext(ctx)
	senderName := ""
	if identity != nil {
		// Use LoginName() for stable identity, fallback to DisplayName()
		if name := identity.LoginName(); name != "" {
			senderName = name
		} else if name := identity.DisplayName(); name != "" {
			senderName = name
		}
	}

	// Add message to buffer and notify subscribers
	messageID, err := s.chatBufferManager.AddUserMessage(req.Page, req.Content, senderName)
	if err != nil {
		if errors.Is(err, chatbuffer.ErrNoSubscribers) {
			// Request an instance from the pool daemon before returning the error
			s.chatBufferManager.RequestInstance(req.Page)
			return nil, status.Error(codes.Unavailable, "no channel subscriber connected")
		}
		return nil, status.Errorf(codes.Internal, "failed to add message: %v", err)
	}

	return &apiv1.SendChatMessageResponse{
		MessageId: messageID,
	}, nil
}

// SubscribeChat implements the SubscribeChat RPC.
// Streams ChatEvents for a given page. Replays existing buffer contents on connect,
// then streams new events (messages, edits, reactions).
func (s *Server) SubscribeChat(req *apiv1.SubscribeChatRequest, stream apiv1.ChatService_SubscribeChatServer) error {
	if req.Page == "" {
		return status.Error(codes.InvalidArgument, errPageRequired)
	}

	// Atomically subscribe and get existing messages to prevent race conditions
	existingMessages, eventChan, unsubscribe := s.chatBufferManager.SubscribeToPageWithReplay(req.Page)
	defer unsubscribe()

	// Replay existing messages
	for _, msg := range existingMessages {
		protoMsg := bufferMessageToProto(msg)
		event := &apiv1.ChatEvent{
			Event: &apiv1.ChatEvent_NewMessage{
				NewMessage: protoMsg,
			},
		}

		if err := stream.Send(event); err != nil {
			return err
		}
	}

	// Stream new events as they arrive
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				return nil
			}

			var protoEvent *apiv1.ChatEvent
			switch event.Type {
			case chatbuffer.EventTypeNewMessage:
				protoEvent = &apiv1.ChatEvent{
					Event: &apiv1.ChatEvent_NewMessage{
						NewMessage: bufferMessageToProto(event.Message),
					},
				}
			case chatbuffer.EventTypeEdit:
				protoEvent = &apiv1.ChatEvent{
					Event: &apiv1.ChatEvent_Edit{
						Edit: &apiv1.ChatMessageEdit{
							MessageId:  event.Edit.MessageID,
							NewContent: event.Edit.NewContent,
							Timestamp:  timestamppb.New(event.Edit.Timestamp),
						},
					},
				}
			case chatbuffer.EventTypeReaction:
				protoEvent = &apiv1.ChatEvent{
					Event: &apiv1.ChatEvent_Reaction{
						Reaction: &apiv1.ChatReaction{
							MessageId: event.Reaction.MessageID,
							Emoji:     event.Reaction.Emoji,
							Reactor:   event.Reaction.Reactor,
						},
					},
				}
			default:
				// Unknown event type — skip without sending
			}

			if protoEvent != nil {
				if err := stream.Send(protoEvent); err != nil {
					return err
				}
			}

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// SubscribeChatMessages implements the SubscribeChatMessages RPC.
// Streams all new user messages across all pages to channel server subscribers.
// This is how wiki-cli mcp receives messages to forward to the channel subscriber.
func (s *Server) SubscribeChatMessages(_ *apiv1.SubscribeChatMessagesRequest, stream apiv1.ChatService_SubscribeChatMessagesServer) error {
	// Subscribe to all user messages
	msgChan, unsubscribe := s.chatBufferManager.SubscribeToChannel()
	defer unsubscribe()

	// Stream messages as they arrive
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				// Channel closed
				return nil
			}

			protoMsg := bufferMessageToProto(msg)
			if err := stream.Send(protoMsg); err != nil {
				return err
			}

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// SendChatReply implements the SendChatReply RPC.
// Called by wiki-cli mcp when the assistant uses the reply tool.
func (s *Server) SendChatReply(_ context.Context, req *apiv1.SendChatReplyRequest) (*apiv1.SendChatReplyResponse, error) {
	if req.Page == "" {
		return nil, status.Error(codes.InvalidArgument, errPageRequired)
	}
	if req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}

	messageID, err := s.chatBufferManager.AddAssistantMessage(req.Page, req.Content, req.ReplyToId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add reply: %v", err)
	}

	return &apiv1.SendChatReplyResponse{
		MessageId: messageID,
	}, nil
}

// EditChatMessage implements the EditChatMessage RPC.
// Called by wiki-cli mcp when the assistant uses the edit_message tool.
func (s *Server) EditChatMessage(_ context.Context, req *apiv1.EditChatMessageRequest) (*apiv1.EditChatMessageResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.NewContent == "" {
		return nil, status.Error(codes.InvalidArgument, "new_content is required")
	}

	err := s.chatBufferManager.EditMessage(req.MessageId, req.NewContent)
	if err != nil {
		if errors.Is(err, chatbuffer.ErrMessageNotFound) {
			return nil, status.Errorf(codes.NotFound, "message not found: %s", req.MessageId)
		}
		return nil, status.Errorf(codes.Internal, "failed to edit message: %v", err)
	}

	return &apiv1.EditChatMessageResponse{}, nil
}

// ReactToMessage implements the ReactToMessage RPC.
// Called by wiki-cli mcp when the assistant uses the react tool.
func (s *Server) ReactToMessage(_ context.Context, req *apiv1.ReactToMessageRequest) (*apiv1.ReactToMessageResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.Emoji == "" {
		return nil, status.Error(codes.InvalidArgument, "emoji is required")
	}

	// Reactor is always "assistant" for channel subscriber reactions
	err := s.chatBufferManager.AddReaction(req.MessageId, req.Emoji, "assistant")
	if err != nil {
		if errors.Is(err, chatbuffer.ErrMessageNotFound) {
			return nil, status.Errorf(codes.NotFound, "message not found: %s", req.MessageId)
		}
		return nil, status.Errorf(codes.Internal, "failed to add reaction: %v", err)
	}

	return &apiv1.ReactToMessageResponse{}, nil
}

// bufferMessageToProto converts a chatbuffer.Message to a protobuf ChatMessage.
func bufferMessageToProto(msg *chatbuffer.Message) *apiv1.ChatMessage {
	var sender apiv1.Sender
	switch msg.Sender {
	case "user":
		sender = apiv1.Sender_USER
	case "assistant":
		sender = apiv1.Sender_ASSISTANT
	default:
		// Unknown sender values map to UNSPECIFIED to avoid silently misrepresenting data
		sender = apiv1.Sender_SENDER_UNSPECIFIED
	}

	reactions := make([]*apiv1.Reaction, len(msg.Reactions))
	for i, r := range msg.Reactions {
		reactions[i] = &apiv1.Reaction{
			Emoji:   r.Emoji,
			Reactor: r.Reactor,
		}
	}

	return &apiv1.ChatMessage{
		Id:         msg.ID,
		Sender:     sender,
		Content:    msg.Content,
		Timestamp:  timestamppb.New(msg.Timestamp),
		Page:       msg.Page,
		Sequence:   msg.Sequence,
		SenderName: msg.SenderName,
		ReplyToId:  msg.ReplyToID,
		Reactions:  reactions,
	}
}

// GetChatStatus implements the GetChatStatus RPC.
// Returns whether a Claude channel subscriber is currently connected.
// If a page is specified, checks for page-specific subscribers. Otherwise falls back to global check.
func (s *Server) GetChatStatus(_ context.Context, req *apiv1.GetChatStatusRequest) (*apiv1.GetChatStatusResponse, error) {
	resp := &apiv1.GetChatStatusResponse{
		PoolConnected: s.chatBufferManager.HasInstanceRequestSubscribers(),
	}

	if req.Page != "" {
		resp.Connected = s.chatBufferManager.HasPageChannelSubscriber(req.Page) || s.chatBufferManager.HasChannelSubscribers()
		resp.Starting = s.chatBufferManager.IsInstanceRequested(req.Page)
	} else {
		resp.Connected = s.chatBufferManager.HasChannelSubscribers()
	}

	return resp, nil
}

// SubscribePageChatMessages implements the SubscribePageChatMessages RPC.
// Streams new user messages for a specific page to per-page Claude instances.
func (s *Server) SubscribePageChatMessages(req *apiv1.SubscribePageChatMessagesRequest, stream apiv1.ChatService_SubscribePageChatMessagesServer) error {
	if req.Page == "" {
		return status.Error(codes.InvalidArgument, errPageRequired)
	}

	msgChan, unsubscribe := s.chatBufferManager.SubscribeToPageChannel(req.Page)
	defer unsubscribe()

	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				return nil
			}

			protoMsg := bufferMessageToProto(msg)
			if err := stream.Send(protoMsg); err != nil {
				return err
			}

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// SubscribeInstanceRequests implements the SubscribeInstanceRequests RPC.
// Streams page names that need a Claude instance spawned. Used by the pool daemon.
func (s *Server) SubscribeInstanceRequests(_ *apiv1.SubscribeInstanceRequestsRequest, stream apiv1.ChatService_SubscribeInstanceRequestsServer) error {
	pageChan, unsubscribe := s.chatBufferManager.SubscribeToInstanceRequests()
	defer unsubscribe()

	for {
		select {
		case page, ok := <-pageChan:
			if !ok {
				return nil
			}

			if err := stream.Send(&apiv1.InstanceRequest{Page: page}); err != nil {
				return err
			}

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
