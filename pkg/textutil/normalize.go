package textutil

import "strings"

// Normalize lowercases s, trims spaces and replaces common Portuguese diacritics
// with their ASCII equivalents. Used for button/state matching throughout the app.
func Normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "á", "a")
	s = strings.ReplaceAll(s, "ã", "a")
	s = strings.ReplaceAll(s, "â", "a")
	s = strings.ReplaceAll(s, "é", "e")
	s = strings.ReplaceAll(s, "ê", "e")
	s = strings.ReplaceAll(s, "í", "i")
	s = strings.ReplaceAll(s, "ó", "o")
	s = strings.ReplaceAll(s, "ô", "o")
	s = strings.ReplaceAll(s, "õ", "o")
	s = strings.ReplaceAll(s, "ú", "u")
	s = strings.ReplaceAll(s, "ç", "c")
	return s
}
