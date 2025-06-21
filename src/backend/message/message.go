package message

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewUserMessage はユーザーからの新しいメッセージを作成します。
func NewUserMessage(content string) Message {
	return Message{Role: "user", Content: content}
}

// NewAssistantMessage はアシスタントからの新しいメッセージを作成します。
func NewAssistantMessage(content string) Message {
	return Message{Role: "assistant", Content: content}
}