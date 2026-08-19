package chats

// ListOptions narrows one caller's chat history without changing ownership.
// A blank Search matches every chat.
type ListOptions struct {
	Search string
}
