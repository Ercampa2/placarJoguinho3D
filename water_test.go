package main

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseDay(t *testing.T) {
	today := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		text string
		want string // "" means invalid
	}{
		{"24/09", "2026-09-24"},
		{"2/9", "2026-09-02"},
		{" 24/09/2025 ", "2025-09-24"},
		{"2026-09-01", "2026-09-01"},
		{"31/02", ""},
		{"ontem", ""},
	}

	for _, tt := range tests {
		got, err := parseDay(tt.text, today)
		if tt.want == "" {
			if err == nil {
				t.Errorf("parseDay(%q) = %q, want an error", tt.text, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("parseDay(%q) = %q, %v; want %q", tt.text, got, err, tt.want)
		}
	}
}

func TestRankingEmbed(t *testing.T) {
	wf := &waterFeature{sched: workSchedule(saoPaulo(t))}
	track := waterTrack{name: "5min", label: "5 min", interval: 5 * time.Minute}

	embed := wf.rankingEmbed(track, "2026-09-21", []rankEntry{{"a", 5}, {"b", 5}, {"c", 3}, {"d", 1}})

	want := "🥇 <@a> — 5\n🥇 <@b> — 5\n🥉 <@c> — 3\n4. <@d> — 1\n"
	if embed.Description != want {
		t.Errorf("description = %q, want %q", embed.Description, want)
	}
	if !strings.Contains(embed.Title, "5 min") {
		t.Errorf("title = %q, want the track label", embed.Title)
	}
	if embed.Footer == nil || embed.Footer.Text != "21/09/2026 · 72 lembretes no dia" {
		t.Errorf("footer = %+v", embed.Footer)
	}

	empty := wf.rankingEmbed(track, "2026-09-21", nil)
	if !strings.Contains(empty.Description, "Ninguém") {
		t.Errorf("empty ranking description = %q", empty.Description)
	}
}

func TestUnknownWaterVars(t *testing.T) {
	got := unknownWaterVars([]string{
		"AGUA_CANAL_5_MIN=1507838687176687877", // typo for AGUA_CANAL_5MIN
		"AGUA_CANAL_5MIN=1",
		"AGUA_TESTE=1",
		"AGUAS=x", // not an AGUA_ variable
		"HOME=/home/x",
	})

	if !slices.Equal(got, []string{"AGUA_CANAL_5_MIN"}) {
		t.Errorf("unknownWaterVars = %v, want [AGUA_CANAL_5_MIN]", got)
	}
}
