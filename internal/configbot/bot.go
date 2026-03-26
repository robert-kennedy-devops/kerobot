package configbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log/slog"

	"kerobot/internal/database"
	"kerobot/internal/models"
)

type Bot struct {
	api       *tgbotapi.BotAPI
	log       *slog.Logger
	repo      *database.ConfigRepo
	adminChat int64
	manager   AccountManager
}

type AccountManager interface {
	StartQRLogin(ctx context.Context, chatID int64) (<-chan []byte, <-chan error)
	StartAccount(ctx context.Context, chatID int64) error
}

func New(token string, adminChatID int64, repo *database.ConfigRepo, log *slog.Logger, manager AccountManager) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{api: api, repo: repo, adminChat: adminChatID, log: log, manager: manager}, nil
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case up := <-updates:
			b.handleUpdate(ctx, up)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, up tgbotapi.Update) {
	if up.Message != nil {
		b.handleMessage(ctx, up.Message)
		return
	}
	if up.CallbackQuery != nil {
		b.handleCallback(ctx, up.CallbackQuery)
		return
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if !b.allowed(msg.Chat.ID) {
		b.reply(msg.Chat.ID, "Acesso não autorizado.")
		return
	}

	_ = EnsureConfig(ctx, b.repo, msg.Chat.ID)

	if msg.Text == "/start" || msg.Text == "/config" {
		if msg.Text == "/start" {
			b.sendIntro(msg.Chat.ID)
		}
		b.sendMenu(ctx, msg.Chat.ID)
		return
	}
	if msg.Text == "/last" {
		b.sendLastCapture(ctx, msg.Chat.ID)
		return
	}
	if strings.HasPrefix(msg.Text, "/learn_last_click ") {
		label := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/learn_last_click"))
		if label == "" {
			b.reply(msg.Chat.ID, "Informe o label do botão. Ex: /learn_last_click Caçar")
			return
		}
		b.learnLastClick(ctx, msg.Chat.ID, label)
		return
	}
	if strings.HasPrefix(msg.Text, "/learn_last_text ") {
		text := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/learn_last_text"))
		if text == "" {
			b.reply(msg.Chat.ID, "Informe o texto gatilho. Ex: /learn_last_text Vitória")
			return
		}
		b.learnLastText(ctx, msg.Chat.ID, text)
		return
	}
	if msg.Text == "/appconfig" {
		b.sendAppConfig(ctx, msg.Chat.ID)
		return
	}
	if msg.Text == "/link" || msg.Text == "/qr" {
		b.sendQR(ctx, msg.Chat.ID)
		return
	}
	if msg.Text == "/capture_on" {
		_ = b.repo.UpdateToggle(ctx, msg.Chat.ID, "capture_enabled", true)
		b.reply(msg.Chat.ID, "Captura ativada.")
		b.sendMenu(ctx, msg.Chat.ID)
		return
	}
	if msg.Text == "/capture_off" {
		_ = b.repo.UpdateToggle(ctx, msg.Chat.ID, "capture_enabled", false)
		b.reply(msg.Chat.ID, "Captura desativada.")
		b.sendMenu(ctx, msg.Chat.ID)
		return
	}

	if strings.HasPrefix(msg.Text, "/set_api ") {
		if !b.allowed(msg.Chat.ID) {
			b.reply(msg.Chat.ID, "Acesso não autorizado.")
			return
		}
		val := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/set_api"))
		if _, err := strconv.Atoi(val); err != nil {
			b.reply(msg.Chat.ID, "API_ID inválido")
			return
		}
		if err := b.repo.SetAppConfig(ctx, "api_id", val); err != nil {
			b.reply(msg.Chat.ID, "Erro ao salvar API_ID")
			return
		}
		b.reply(msg.Chat.ID, "API_ID atualizado")
		b.tryStartAccount(ctx, msg.Chat.ID)
		b.sendAppConfig(ctx, msg.Chat.ID)
		return
	}

	if strings.HasPrefix(msg.Text, "/set_hash ") {
		if !b.allowed(msg.Chat.ID) {
			b.reply(msg.Chat.ID, "Acesso não autorizado.")
			return
		}
		val := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/set_hash"))
		if len(val) < 16 {
			b.reply(msg.Chat.ID, "API_HASH inválido")
			return
		}
		if err := b.repo.SetAppConfig(ctx, "api_hash", val); err != nil {
			b.reply(msg.Chat.ID, "Erro ao salvar API_HASH")
			return
		}
		b.reply(msg.Chat.ID, "API_HASH atualizado")
		b.tryStartAccount(ctx, msg.Chat.ID)
		b.sendAppConfig(ctx, msg.Chat.ID)
		return
	}

	if strings.HasPrefix(msg.Text, "/set_heal ") {
		p := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/set_heal"))
		val, _ := strconv.Atoi(strings.TrimSpace(p))
		if val < 1 || val > 100 {
			b.reply(msg.Chat.ID, "Valor inválido (1-100)")
			return
		}
		_ = b.repo.UpdateHealPercent(ctx, msg.Chat.ID, val)
		b.reply(msg.Chat.ID, fmt.Sprintf("heal_percent = %d", val))
		b.sendMenu(ctx, msg.Chat.ID)
		return
	}

	if strings.HasPrefix(msg.Text, "/set_min_potions ") {
		p := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/set_min_potions"))
		val, _ := strconv.Atoi(strings.TrimSpace(p))
		if val < 0 || val > 999 {
			b.reply(msg.Chat.ID, "Valor inválido")
			return
		}
		_ = b.repo.UpdateMinPotions(ctx, msg.Chat.ID, val)
		b.reply(msg.Chat.ID, fmt.Sprintf("min_potions = %d", val))
		b.sendMenu(ctx, msg.Chat.ID)
	}
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if !b.allowed(cb.Message.Chat.ID) {
		b.answer(cb.ID, "Acesso não autorizado")
		return
	}

	_ = EnsureConfig(ctx, b.repo, cb.Message.Chat.ID)

	data := cb.Data
	if data == "last" {
		b.answer(cb.ID, "Buscando...")
		b.sendLastCapture(ctx, cb.Message.Chat.ID)
		return
	}
	if data == "learn:click" {
		b.answer(cb.ID, "Use /learn_last_click <label>")
		return
	}
	if data == "learn:text" {
		b.answer(cb.ID, "Use /learn_last_text <texto>")
		return
	}
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		b.answer(cb.ID, "Comando inválido")
		return
	}

	switch parts[0] {
	case "login":
		if len(parts) >= 2 && parts[1] == "qr" {
			b.answer(cb.ID, "Gerando QR...")
			b.sendQR(ctx, cb.Message.Chat.ID)
			return
		}
	case "last":
		b.answer(cb.ID, "Buscando...")
		b.sendLastCapture(ctx, cb.Message.Chat.ID)
		return
	case "toggle":
		field := parts[1]
		cfg, err := b.repo.GetConfig(ctx, cb.Message.Chat.ID)
		if err != nil {
			b.answer(cb.ID, "Erro ao ler config")
			return
		}
		var newVal bool
		switch field {
		case "auto_heal":
			newVal = !cfg.AutoHeal
		case "auto_buy_potions":
			newVal = !cfg.AutoBuyPotions
		case "auto_dungeon":
			newVal = !cfg.AutoDungeon
		case "auto_hunt":
			newVal = !cfg.AutoHunt
		case "auto_combat":
			newVal = !cfg.AutoCombat
		case "capture_enabled":
			newVal = !cfg.CaptureEnabled
		default:
			b.answer(cb.ID, "Campo inválido")
			return
		}
		_ = b.repo.UpdateToggle(ctx, cb.Message.Chat.ID, field, newVal)
		b.answer(cb.ID, "Atualizado")
		b.sendMenu(ctx, cb.Message.Chat.ID)
	case "set":
		if len(parts) != 3 {
			b.answer(cb.ID, "Comando inválido")
			return
		}
		key := parts[1]
		val, _ := strconv.Atoi(parts[2])
		switch key {
		case "heal":
			_ = b.repo.UpdateHealPercent(ctx, cb.Message.Chat.ID, val)
		case "min":
			_ = b.repo.UpdateMinPotions(ctx, cb.Message.Chat.ID, val)
		default:
			b.answer(cb.ID, "Comando inválido")
			return
		}
		b.answer(cb.ID, "Atualizado")
		b.sendMenu(ctx, cb.Message.Chat.ID)
	}
}

func (b *Bot) sendMenu(ctx context.Context, chatID int64) {
	cfg, err := b.repo.GetConfig(ctx, chatID)
	if err != nil {
		b.reply(chatID, "Erro ao ler config")
		return
	}

	apiID, _ := b.repo.GetAppConfig(ctx, "api_id")
	apiHash, _ := b.repo.GetAppConfig(ctx, "api_hash")
	appReady := apiID != "" && apiHash != ""

	text := fmt.Sprintf(
		"Configuração\nApp API: %v\nHeal%%: %d\nMinPotions: %d\nAutoHeal: %v\nAutoBuy: %v\nAutoDungeon: %v\nAutoHunt: %v\nAutoCombat: %v\nCapture: %v",
		appReady,
		cfg.HealPercent, cfg.MinPotions, cfg.AutoHeal, cfg.AutoBuyPotions, cfg.AutoDungeon, cfg.AutoHunt, cfg.AutoCombat, cfg.CaptureEnabled,
	)

	autoHealLabel := "AutoHeal OFF"
	if cfg.AutoHeal {
		autoHealLabel = "AutoHeal ON"
	}
	autoBuyLabel := "AutoBuy OFF"
	if cfg.AutoBuyPotions {
		autoBuyLabel = "AutoBuy ON"
	}
	autoDungeonLabel := "AutoDungeon OFF"
	if cfg.AutoDungeon {
		autoDungeonLabel = "AutoDungeon ON"
	}
	autoHuntLabel := "AutoHunt OFF"
	if cfg.AutoHunt {
		autoHuntLabel = "AutoHunt ON"
	}
	autoCombatLabel := "AutoCombat OFF"
	if cfg.AutoCombat {
		autoCombatLabel = "AutoCombat ON"
	}
	captureLabel := "Capture OFF"
	if cfg.CaptureEnabled {
		captureLabel = "Capture ON"
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(autoHealLabel, "toggle:auto_heal"),
			tgbotapi.NewInlineKeyboardButtonData(autoBuyLabel, "toggle:auto_buy_potions"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(autoDungeonLabel, "toggle:auto_dungeon"),
			tgbotapi.NewInlineKeyboardButtonData(autoHuntLabel, "toggle:auto_hunt"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(autoCombatLabel, "toggle:auto_combat"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(captureLabel, "toggle:capture_enabled"),
			tgbotapi.NewInlineKeyboardButtonData("Last", "last"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Learn Click", "learn:click"),
			tgbotapi.NewInlineKeyboardButtonData("Learn Text", "learn:text"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Heal 40", "set:heal:40"),
			tgbotapi.NewInlineKeyboardButtonData("Heal 60", "set:heal:60"),
			tgbotapi.NewInlineKeyboardButtonData("Heal 90", "set:heal:90"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Min 1", "set:min:1"),
			tgbotapi.NewInlineKeyboardButtonData("Min 5", "set:min:5"),
			tgbotapi.NewInlineKeyboardButtonData("Min 10", "set:min:10"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Login QR", "login:qr"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	_, _ = b.api.Send(msg)
}

func (b *Bot) sendIntro(chatID int64) {
	text := "Bem-vindo ao KeroBot.\n\n" +
		"Como obter API_ID e API_HASH:\n" +
		"1) Acesse https://my.telegram.org\n" +
		"2) Faça login com seu número\n" +
		"3) Vá em API development tools\n" +
		"4) Crie um app e copie API_ID e API_HASH\n\n" +
		"Sequência recomendada:\n" +
		"1) /set_api <id>\n" +
		"2) /set_hash <hash>\n" +
		"3) /qr (escaneie no Telegram)\n" +
		"4) /config para ajustar automações\n"
	b.reply(chatID, text)
}

func (b *Bot) sendAppConfig(ctx context.Context, chatID int64) {
	apiID, _ := b.repo.GetAppConfig(ctx, "api_id")
	apiHash, _ := b.repo.GetAppConfig(ctx, "api_hash")
	status := fmt.Sprintf("App config\nAPI_ID: %v\nAPI_HASH: %v\n\nUse /set_api <id> and /set_hash <hash>.",
		mask(apiID), mask(apiHash))
	b.reply(chatID, status)
}

func (b *Bot) sendLastCapture(ctx context.Context, chatID int64) {
	c, err := b.repo.LastCapture(ctx, chatID)
	if err != nil {
		b.reply(chatID, "Nenhuma captura encontrada.")
		return
	}
	text := c.Text
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	btns := "[]"
	if len(c.Buttons) > 0 {
		btns = strings.Join(c.Buttons, ", ")
	}
	msg := fmt.Sprintf("Última captura\nEstado: %s\nHP: %d\nPoções: %d\nBotões: %s\nTexto:\n%s", c.State, c.HPPercent, c.Potions, btns, text)
	b.reply(chatID, msg)
}

func (b *Bot) learnLastClick(ctx context.Context, chatID int64, label string) {
	c, err := b.repo.LastCapture(ctx, chatID)
	if err != nil {
		b.reply(chatID, "Nenhuma captura encontrada.")
		return
	}
	found := false
	for _, b0 := range c.Buttons {
		if strings.EqualFold(b0, label) {
			found = true
			label = b0
			break
		}
	}
	if !found {
		b.reply(chatID, "Botão não encontrado na última captura.")
		return
	}
	rule := models.Rule{
		TelegramID:  chatID,
		MatchType:   "button",
		MatchValue:  label,
		ActionType:  "click",
		ActionValue: label,
		Enabled:     true,
	}
	if err := b.repo.SaveRule(ctx, rule); err != nil {
		b.reply(chatID, "Erro ao salvar regra.")
		return
	}
	b.reply(chatID, "Regra criada: quando o botão aparecer, clicar.")
}

func (b *Bot) learnLastText(ctx context.Context, chatID int64, text string) {
	c, err := b.repo.LastCapture(ctx, chatID)
	if err != nil {
		b.reply(chatID, "Nenhuma captura encontrada.")
		return
	}
	var label string
	for _, b0 := range c.Buttons {
		if strings.Contains(strings.ToLower(b0), strings.ToLower(text)) {
			label = b0
			break
		}
	}
	if label == "" {
		b.reply(chatID, "Nenhum botão compatível encontrado na última captura.")
		return
	}
	rule := models.Rule{
		TelegramID:  chatID,
		MatchType:   "text_contains",
		MatchValue:  text,
		ActionType:  "click",
		ActionValue: label,
		Enabled:     true,
	}
	if err := b.repo.SaveRule(ctx, rule); err != nil {
		b.reply(chatID, "Erro ao salvar regra.")
		return
	}
	b.reply(chatID, "Regra criada: quando o texto aparecer, clicar no botão correspondente.")
}

func (b *Bot) tryStartAccount(ctx context.Context, chatID int64) {
	if b.manager == nil {
		return
	}
	if err := b.manager.StartAccount(ctx, chatID); err != nil {
		b.log.Error("start account failed", slog.Any("err", err), slog.Int64("chat_id", chatID))
	}
}

func (b *Bot) sendQR(ctx context.Context, chatID int64) {
	if b.manager == nil {
		b.reply(chatID, "QR login indisponível.")
		return
	}
	qrCh, errCh := b.manager.StartQRLogin(ctx, chatID)
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if ok && err != nil {
				b.reply(chatID, fmt.Sprintf("Erro ao gerar QR: %v", err))
			}
			return
		case img, ok := <-qrCh:
			if !ok {
				return
			}
			file := tgbotapi.FileBytes{Name: "qr.png", Bytes: img}
			photo := tgbotapi.NewPhoto(chatID, file)
			photo.Caption = "Escaneie este QR no Telegram para autenticar."
			_, _ = b.api.Send(photo)
		}
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, _ = b.api.Send(msg)
}

func (b *Bot) answer(cbID, text string) {
	cb := tgbotapi.NewCallback(cbID, text)
	_, _ = b.api.Request(cb)
}

func (b *Bot) allowed(chatID int64) bool {
	if b.adminChat == 0 {
		return true
	}
	return chatID == b.adminChat
}

// EnsureConfig makes sure there's a config row for the chat user.
func EnsureConfig(ctx context.Context, repo *database.ConfigRepo, chatID int64) error {
	return repo.EnsureUserAndConfig(ctx, chatID)
}

func mask(v string) string {
	if v == "" {
		return "not set"
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + "****" + v[len(v)-2:]
}
