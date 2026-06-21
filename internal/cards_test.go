package internal

import (
	"path/filepath"
	"testing"
)

func TestAddCardPersists(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom.json")
	d := &Deck{customPath: custom}

	pick, err := d.AddCard("black", "Why did the _ cross the __ road?")
	if err != nil {
		t.Fatal(err)
	}
	if pick != 2 {
		t.Fatalf("black pick = %d, want 2", pick)
	}
	if pick, _ := d.AddCard("white", "A custom card."); pick != 1 {
		t.Fatalf("white pick = %d, want 1", pick)
	}

	if got := len(d.ShuffledWhite()); got != 1 {
		t.Fatalf("white in deck = %d, want 1", got)
	}
	if got := len(d.ShuffledBlack()); got != 1 {
		t.Fatalf("black in deck = %d, want 1", got)
	}

	reloaded, err := readCards(custom)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.White) != 1 || len(reloaded.Black) != 1 {
		t.Fatalf("persisted white=%d black=%d", len(reloaded.White), len(reloaded.Black))
	}
	if reloaded.Black[0].Pick != 2 {
		t.Fatalf("persisted pick = %d", reloaded.Black[0].Pick)
	}
}

func TestLoadDeckMergesCustom(t *testing.T) {
	d, err := LoadDeck("../cards/cards.json", "../cards/custom_cards.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.ShuffledWhite()) < 1000 || len(d.ShuffledBlack()) < 200 {
		t.Fatalf("base deck looks wrong: white=%d black=%d", len(d.ShuffledWhite()), len(d.ShuffledBlack()))
	}
}
