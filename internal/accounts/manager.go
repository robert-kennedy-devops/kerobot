package accounts

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	tdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"log/slog"
	"rsc.io/qr"

	"kerobot/internal/automation"
	"kerobot/internal/config"
	"kerobot/internal/database"
	"kerobot/internal/engine"
	"kerobot/internal/models"
	"kerobot/internal/parser"
	internaltelegram "kerobot/internal/telegram"
	"kerobot/pkg/configcache"
	"kerobot/pkg/metrics"
	"kerobot/pkg/retry"
)

type Manager struct {
	cfg      config.Config
	repo     *database.ConfigRepo
	log      *slog.Logger
	mu       sync.Mutex
	runs     map[int64]context.CancelFunc
	counters *metrics.Counters
	capMu    sync.Mutex
	capCache map[int64]capEntry
}

type capEntry struct {
	enabled bool
	exp     time.Time
}

func NewManager(cfg config.Config, repo *database.ConfigRepo, log *slog.Logger, counters *metrics.Counters) *Manager {
	return &Manager{cfg: cfg, repo: repo, log: log, runs: make(map[int64]context.CancelFunc), counters: counters, capCache: make(map[int64]capEntry)}
}

func (m *Manager) StartExisting(ctx context.Context) {
	dir := m.sessionDir()
	_ = os.MkdirAll(dir, 0o755)
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.log.Error("session dir read failed", slog.Any("err", err))
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".session") {
			continue
		}
		idStr := strings.TrimSuffix(name, ".session")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		_ = m.StartAccount(ctx, id)
	}
}

func (m *Manager) StartAccount(ctx context.Context, telegramID int64) error {
	m.mu.Lock()
	if _, ok := m.runs[telegramID]; ok {
		m.mu.Unlock()
		return nil
	}
	accCtx, cancel := context.WithCancel(ctx)
	m.runs[telegramID] = cancel
	m.mu.Unlock()

	sessionPath := m.sessionPath(telegramID)
	go m.runAccount(accCtx, telegramID, sessionPath)
	return nil
}

func (m *Manager) StartQRLogin(ctx context.Context, chatID int64) ([]byte, error) {
	// Ensure config row
	_ = m.repo.EnsureUserAndConfig(ctx, chatID)

	sessionPath := m.sessionPath(chatID)
	apiID, apiHash, err := m.appCreds(ctx)
	if err != nil {
		return nil, err
	}
	d := tg.NewUpdateDispatcher()
	loggedIn := qrlogin.OnLoginToken(d)
	updatesHandler := updates.New(updates.Config{Handler: d})

	client := tdtelegram.NewClient(apiID, apiHash, tdtelegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
		UpdateHandler:  updatesHandler,
	})

	qrCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		err := client.Run(ctx, func(ctx context.Context) error {
			qrAuth := client.QR()
			_, err := qrAuth.Auth(ctx, loggedIn, func(ctx context.Context, token qrlogin.Token) error {
				img, err := renderQR(token.URL(), 640)
				if err != nil {
					return err
				}
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					return err
				}
				select {
				case qrCh <- buf.Bytes():
				default:
				}
				m.log.Info("qr generated", slog.Int64("chat_id", chatID), slog.String("expires", token.Expires().Format(time.RFC3339)))
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			errCh <- err
			return
		}
		// Successful login: start account
		_ = m.StartAccount(ctx, chatID)
	}()

	select {
	case img := <-qrCh:
		return img, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("qr timeout")
	}
}

func (m *Manager) sessionDir() string {
	if m.cfg.Telegram.SessionDir != "" {
		return m.cfg.Telegram.SessionDir
	}
	return "./data/sessions"
}

func (m *Manager) sessionPath(telegramID int64) string {
	return filepath.Join(m.sessionDir(), fmt.Sprintf("%d.session", telegramID))
}

func (m *Manager) appCreds(ctx context.Context) (int, string, error) {
	if m.cfg.Telegram.APIID != 0 && m.cfg.Telegram.APIHash != "" {
		return m.cfg.Telegram.APIID, m.cfg.Telegram.APIHash, nil
	}
	apiIDStr, err := m.repo.GetAppConfig(ctx, "api_id")
	if err != nil {
		return 0, "", fmt.Errorf("read api_id: %w", err)
	}
	apiHash, err := m.repo.GetAppConfig(ctx, "api_hash")
	if err != nil {
		return 0, "", fmt.Errorf("read api_hash: %w", err)
	}
	if apiIDStr == "" || apiHash == "" {
		return 0, "", fmt.Errorf("API_ID/API_HASH not configured. Use the config bot to set them")
	}
	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil || apiID <= 0 {
		return 0, "", fmt.Errorf("invalid api_id: %s", apiIDStr)
	}
	return apiID, apiHash, nil
}

func (m *Manager) runAccount(ctx context.Context, telegramID int64, sessionPath string) {
	m.log.Info("account start", slog.Int64("telegram_id", telegramID))
	_ = m.repo.EnsureUserAndConfig(ctx, telegramID)

	apiID, apiHash, err := m.appCreds(ctx)
	if err != nil {
		m.log.Error("telegram api credentials missing", slog.Any("err", err), slog.Int64("telegram_id", telegramID))
		return
	}

	tgClient := internaltelegram.NewClient(
		apiID,
		apiHash,
		"",
		m.cfg.Telegram.Password,
		"",
		sessionPath,
		m.cfg.Limits.RatePerSecond,
	)

	listener := internaltelegram.NewListener(0)

	go func() {
		if err := tgClient.Start(ctx, listener.Handle); err != nil {
			m.log.Error("telegram client stopped", slog.Any("err", err), slog.Int64("telegram_id", telegramID))
		}
	}()

	select {
	case <-tgClient.Ready():
		m.log.Info("telegram ready", slog.Int64("telegram_id", telegramID))
	case <-time.After(60 * time.Second):
		m.log.Error("telegram not ready", slog.Int64("telegram_id", telegramID))
		return
	}

	targetPeer, err := tgClient.ResolvePeerByUsername(ctx, m.cfg.Telegram.TargetBot)
	if err != nil {
		m.log.Error("resolve target bot failed", slog.Any("err", err), slog.Int64("telegram_id", telegramID))
		return
	}
	listener.SetTarget(internaltelegram.PeerID(targetPeer))

	_ = retry.Do(ctx, retry.Config{Attempts: 3, Delay: 2 * time.Second}, func() error {
		return tgClient.SendMessage(ctx, targetPeer, "/start")
	})

	state := engine.NewStateManager()
	exec := engine.NewExecutor(tgClient, m.log, m.cfg.Limits.ClickDelay, retry.Config{Attempts: m.cfg.Limits.RetryAttempts, Delay: m.cfg.Limits.RetryDelay}, m.counters)
	exec.Start(ctx)

	cachedCfg := configcache.New(m.repo, 30*time.Second)
	automEngine := engine.NewAutomationEngine(m.log, state, exec, cachedCfg, m.repo, telegramID)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-listener.Events():
				if msg == nil {
					continue
				}
				buttons := extractButtonLabels(msg)
				snapshot := parser.Parse(msg.Text, buttons)
				m.captureSnapshot(ctx, telegramID, snapshot)
				automEngine.HandleSnapshot(ctx, snapshot, targetPeer)
			}
		}
	}()

	go automation.NewAutoHuntWorker(state, exec.Queue(), targetPeer, m.cfg.Automation.HuntInterval, cachedCfg, telegramID, m.log).Run(ctx)
	go automation.NewAutoCombatWorker(state, exec.Queue(), targetPeer, m.cfg.Automation.CombatInterval, cachedCfg, telegramID, m.log).Run(ctx)
	go automation.NewAutoHealWorker(state, exec.Queue(), targetPeer, m.cfg.Automation.HealInterval, m.cfg.Automation.HealPercent, cachedCfg, telegramID, m.log).Run(ctx)
	go automation.NewAutoPotionWorker(state, exec.Queue(), targetPeer, m.cfg.Automation.PotionInterval, m.cfg.Automation.MinPotions, cachedCfg, telegramID, m.log).Run(ctx)
	go automation.NewDungeonWorker(state, exec.Queue(), targetPeer, m.cfg.Automation.DungeonInterval, cachedCfg, telegramID, m.log).Run(ctx)

	<-ctx.Done()
	m.log.Info("account stopped", slog.Int64("telegram_id", telegramID))
}

func extractButtonLabels(msg *internaltelegram.Message) []string {
	if msg == nil {
		return nil
	}
	labels := make([]string, 0, len(msg.Buttons))
	for _, b := range msg.Buttons {
		labels = append(labels, b.Text)
	}
	return labels
}

func (m *Manager) captureSnapshot(ctx context.Context, telegramID int64, snapshot parser.Snapshot) {
	if telegramID == 0 || m.repo == nil {
		return
	}
	if !m.captureEnabled(ctx, telegramID) {
		return
	}
	c := models.Capture{
		TelegramID: telegramID,
		State:      string(snapshot.State),
		Text:       snapshot.Text,
		Buttons:    snapshot.Buttons,
		HPPercent:  snapshot.HPPercent,
		Potions:    snapshot.Potions,
	}
	if err := m.repo.SaveCapture(ctx, c); err != nil {
		m.log.Debug("save capture failed", slog.Any("err", err), slog.Int64("telegram_id", telegramID))
	}
}

func (m *Manager) captureEnabled(ctx context.Context, telegramID int64) bool {
	m.capMu.Lock()
	if ent, ok := m.capCache[telegramID]; ok && time.Now().Before(ent.exp) {
		m.capMu.Unlock()
		return ent.enabled
	}
	m.capMu.Unlock()

	enabled := false
	if cfg, err := m.repo.GetConfig(ctx, telegramID); err == nil {
		enabled = cfg.CaptureEnabled
	}
	m.capMu.Lock()
	m.capCache[telegramID] = capEntry{enabled: enabled, exp: time.Now().Add(30 * time.Second)}
	m.capMu.Unlock()
	return enabled
}

func renderQR(url string, size int) (image.Image, error) {
	if size <= 0 {
		size = 640
	}
	code, err := qr.Encode(url, qr.M)
	if err != nil {
		return nil, err
	}
	modules := code.Size
	if modules <= 0 {
		return nil, fmt.Errorf("invalid qr size")
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)

	for y := 0; y < size; y++ {
		my := (y * modules) / size
		for x := 0; x < size; x++ {
			mx := (x * modules) / size
			if code.Black(mx, my) {
				dst.Set(x, y, color.Black)
			}
		}
	}
	return dst, nil
}

