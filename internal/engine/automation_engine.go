package engine

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gotd/td/tg"

	"kerobot/internal/models"
	"kerobot/internal/parser"
)

type AutomationEngine struct {
	log        *slog.Logger
	state      *StateManager
	executor   *Executor
	cfgReader  ConfigReader
	ruleReader RuleReader
	telegramID int64
}

type ConfigReader interface {
	GetConfig(ctx context.Context, telegramID int64) (models.Config, error)
}

type RuleReader interface {
	ListRules(ctx context.Context, telegramID int64) ([]models.Rule, error)
}

func NewAutomationEngine(log *slog.Logger, state *StateManager, executor *Executor, cfgReader ConfigReader, ruleReader RuleReader, telegramID int64) *AutomationEngine {
	return &AutomationEngine{log: log, state: state, executor: executor, cfgReader: cfgReader, ruleReader: ruleReader, telegramID: telegramID}
}

func (e *AutomationEngine) HandleSnapshot(ctx context.Context, snapshot parser.Snapshot, targetPeer tg.InputPeerClass) {
	e.state.Update(snapshot)
	if e.handleLearnedRules(ctx, snapshot, targetPeer) {
		return
	}
	allowHunt := true
	allowCombat := true
	if e.cfgReader != nil && e.telegramID != 0 {
		if cfg, err := e.cfgReader.GetConfig(ctx, e.telegramID); err == nil {
			allowHunt = cfg.AutoHunt
			allowCombat = cfg.AutoCombat
		} else {
			e.log.Debug("engine config read failed", slog.Any("err", err))
		}
	}
	switch snapshot.State {
	case parser.StateCombat:
		if allowCombat {
			e.enqueue(Action{Type: ActionClick, Label: "Atacar", Peer: targetPeer, Reason: "combat"})
		}
	case parser.StateMainMenu:
		if allowHunt {
			e.enqueue(Action{Type: ActionClick, Label: "Caçar", Peer: targetPeer, Reason: "main_menu"})
		}
	case parser.StateVictory:
		if allowHunt {
			e.enqueue(Action{Type: ActionClick, Label: "Caçar de novo", Peer: targetPeer, Reason: "victory"})
		}
	}
}

func (e *AutomationEngine) enqueue(action Action) {
	e.executor.Queue() <- action
	e.log.Debug("action queued", slog.String("action", string(action.Type)), slog.String("label", action.Label))
}

func (e *AutomationEngine) handleLearnedRules(ctx context.Context, snapshot parser.Snapshot, targetPeer tg.InputPeerClass) bool {
	if e.ruleReader == nil || e.telegramID == 0 {
		return false
	}
	rules, err := e.ruleReader.ListRules(ctx, e.telegramID)
	if err != nil || len(rules) == 0 {
		return false
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !matchRule(snapshot, r) {
			continue
		}
		switch r.ActionType {
		case "click":
			e.enqueue(Action{Type: ActionClick, Label: r.ActionValue, Peer: targetPeer, Reason: "rule"})
			return true
		case "send":
			e.enqueue(Action{Type: ActionSend, Text: r.ActionValue, Peer: targetPeer, Reason: "rule"})
			return true
		}
	}
	return false
}

func matchRule(snapshot parser.Snapshot, r models.Rule) bool {
	switch r.MatchType {
	case "button":
		for _, b := range snapshot.Buttons {
			if norm(b) == norm(r.MatchValue) {
				return true
			}
		}
		return false
	case "text_contains":
		return strings.Contains(norm(snapshot.Text), norm(r.MatchValue))
	case "state":
		return norm(string(snapshot.State)) == norm(r.MatchValue)
	default:
		return false
	}
}

func norm(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	repl := strings.NewReplacer(
		"á", "a", "ã", "a", "â", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "ô", "o", "õ", "o",
		"ú", "u",
		"ç", "c",
	)
	return repl.Replace(s)
}
