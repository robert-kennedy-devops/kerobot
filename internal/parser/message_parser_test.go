package parser

import "testing"

func TestParseStatePrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		buttons []string
		want    GameState
	}{
		{
			name:    "victory_over_hunting",
			text:    "Você ganhou! Vitória! Continue caçando.",
			buttons: []string{"Caçar"},
			want:    StateVictory,
		},
		{
			name:    "combat_over_menu",
			text:    "Um inimigo apareceu.",
			buttons: []string{"Caçar", "Atacar"},
			want:    StateCombat,
		},
		{
			name:    "defeat_over_menu",
			text:    "Derrota... tente novamente.",
			buttons: []string{"Caçar"},
			want:    StateDefeat,
		},
		{
			name:    "inventory_when_button_or_text",
			text:    "Abrindo inventario",
			buttons: []string{"Inventário"},
			want:    StateInventory,
		},
		{
			name:    "dungeon_when_button_present",
			text:    "Prepare-se para a masmorra",
			buttons: []string{"Masmorra"},
			want:    StateDungeon,
		},
		{
			name:    "hunting_when_text_only",
			text:    "Você está caçando na floresta",
			buttons: []string{},
			want:    StateHunting,
		},
		{
			name:    "main_menu_when_hunt_button_only",
			text:    "Menu principal",
			buttons: []string{"Caçar"},
			want:    StateMainMenu,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Parse(tt.text, tt.buttons)
			if got.State != tt.want {
				t.Fatalf("state=%s want=%s", got.State, tt.want)
			}
		})
	}
}

func TestParseHPAndPotions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		wantHP    int
		wantPots  int
	}{
		{
			name:     "hp_percent_simple",
			text:     "HP: 35%",
			wantHP:   35,
			wantPots: -1,
		},
		{
			name:     "hp_fraction",
			text:     "HP 120/300",
			wantHP:   40,
			wantPots: -1,
		},
		{
			name:     "potions_poçoes",
			text:     "Poções: 7",
			wantHP:   0,
			wantPots: 7,
		},
		{
			name:     "potions_estoque",
			text:     "Estoque: 3",
			wantHP:   0,
			wantPots: 3,
		},
		{
			name:     "potions_item_count",
			text:     "Poção de Vida x12",
			wantHP:   0,
			wantPots: 12,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Parse(tt.text, nil)
			if got.HPPercent != tt.wantHP {
				t.Fatalf("hp=%d want=%d", got.HPPercent, tt.wantHP)
			}
			if got.Potions != tt.wantPots {
				t.Fatalf("potions=%d want=%d", got.Potions, tt.wantPots)
			}
		})
	}
}
