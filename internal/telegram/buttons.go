package telegram

import "github.com/gotd/td/tg"

func extractButtons(m *tg.Message) []InlineButton {
	var buttons []InlineButton
	if markup, ok := m.ReplyMarkup.(*tg.ReplyInlineMarkup); ok {
		for _, row := range markup.Rows {
			for _, btn := range row.Buttons {
				if cb, ok := btn.(*tg.KeyboardButtonCallback); ok {
					buttons = append(buttons, InlineButton{Text: cb.Text, Data: cb.Data})
				}
			}
		}
	}
	return buttons
}
