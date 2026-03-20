package parser

import (
	"regexp"
	"strings"

	"kerobot/pkg/textutil"
)

var (
	hpRegex         = regexp.MustCompile(`(?i)hp\s*:?\s*(\d{1,3})%`)
	hpFracRegex     = regexp.MustCompile(`(?i)hp\s*:?\s*(\d+)\s*/\s*(\d+)`)
	potRegex        = regexp.MustCompile(`(?i)po[cç]ões?\s*:?\s*(\d+)`)
	stockRegex      = regexp.MustCompile(`(?i)estoque\s*:?\s*(\d+)`)
	potionItemRegex = regexp.MustCompile(`(?i)po[cç]ao de vida\s*(?:x|×|:)?\s*(\d+)`)
)

func Parse(text string, buttons []string) Snapshot {
	lower := strings.ToLower(text)
	snapshot := Snapshot{State: StateUnknown, Text: text, Buttons: buttons, Potions: -1}

	if hp := hpRegex.FindStringSubmatch(lower); len(hp) == 2 {
		snapshot.HPPercent = atoiSafe(hp[1])
	}
	if snapshot.HPPercent == 0 {
		if hp := hpFracRegex.FindStringSubmatch(lower); len(hp) == 3 {
			cur := atoiSafe(hp[1])
			max := atoiSafe(hp[2])
			if max > 0 {
				snapshot.HPPercent = int(float64(cur) / float64(max) * 100.0)
			}
		}
	}
	if pot := potRegex.FindStringSubmatch(lower); len(pot) == 2 {
		snapshot.Potions = atoiSafe(pot[1])
	}
	if snapshot.Potions < 0 {
		if stock := stockRegex.FindStringSubmatch(lower); len(stock) == 2 {
			snapshot.Potions = atoiSafe(stock[1])
		}
	}
	if snapshot.Potions < 0 {
		if match := potionItemRegex.FindStringSubmatch(lower); len(match) == 2 {
			snapshot.Potions = atoiSafe(match[1])
		}
	}

	if containsButton(buttons, "caçar") {
		snapshot.State = StateMainMenu
	}
	if containsButton(buttons, "atacar") {
		snapshot.State = StateCombat
	}
	if containsButton(buttons, "inventário") || strings.Contains(textutil.Normalize(lower), "inventario") {
		snapshot.State = StateInventory
	}
	if containsButton(buttons, "dungeon") || containsButton(buttons, "masmorra") || strings.Contains(lower, "dungeon") || strings.Contains(lower, "masmorra") {
		snapshot.State = StateDungeon
	}
	if strings.Contains(lower, "vitória") {
		snapshot.State = StateVictory
	}
	if strings.Contains(lower, "derrota") {
		snapshot.State = StateDefeat
	}
	if strings.Contains(lower, "caçando") {
		snapshot.State = StateHunting
	}

	return snapshot
}

func containsButton(buttons []string, label string) bool {
	target := textutil.Normalize(label)
	for _, b := range buttons {
		if n := textutil.Normalize(b); n == target || strings.Contains(n, target) {
			return true
		}
	}
	return false
}

// HasButton checks if a button label exists using normalized/contains matching.
func HasButton(buttons []string, label string) bool {
	return containsButton(buttons, label)
}

// Normalize exposes textutil.Normalize for callers that import only this package.
func Normalize(s string) string {
	return textutil.Normalize(s)
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			continue
		}
		n = n*10 + int(r-'0')
	}
	return n
}
