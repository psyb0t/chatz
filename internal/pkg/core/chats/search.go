package chats

import "strings"

const (
	MaxChatSearchRunes = 256
	likeEscape         = "!"
)

// ChatSearch normalizes a title search before passing it into the generated
// parameterized query. A blank return means no search filter.
func ChatSearch(search string) string {
	trimmed := strings.TrimSpace(search)

	runes := []rune(trimmed)
	if len(runes) > MaxChatSearchRunes {
		return string(runes[:MaxChatSearchRunes])
	}

	return string(runes)
}

func chatSearchPattern(search string) string {
	search = ChatSearch(search)
	if search == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		likeEscape,
		likeEscape+likeEscape,
		"%",
		likeEscape+"%",
		"_",
		likeEscape+"_",
	)

	return "%" + replacer.Replace(search) + "%"
}
