package internal

import (
	"strings"
	"testing"
)

func TestCountPicks(t *testing.T) {
	cases := map[string]int{
		"_ + _ = Hipsters":      2,
		"No blanks here.":       1,
		"Why __? Because __.":   2,
		"a__b__c__d":            3,
		"_____ single long run": 1,
	}
	for text, want := range cases {
		if got := CountPicks(text); got != want {
			t.Errorf("CountPicks(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestFillBlack(t *testing.T) {
	got := fillBlack("_ + _ = Hipsters", []string{"Bees", "War"})
	want := "__**Bees**__ + __**War**__ = Hipsters"
	if got != want {
		t.Errorf("fillBlack = %q, want %q", got, want)
	}

	// No blanks: the answer is appended.
	got = fillBlack("What a day.", []string{"Cheese"})
	if !strings.Contains(got, formatAnswer("Cheese")) || !strings.HasPrefix(got, "What a day.") {
		t.Errorf("fillBlack appended = %q", got)
	}
}

func testDeck() *Deck {
	white := make([]string, 0, 100)
	for i := 'a'; i <= 'z'; i++ {
		for j := 0; j < 5; j++ {
			white = append(white, string(i)+string(rune('0'+j)))
		}
	}
	return &Deck{
		white: white,
		black: []BlackCard{{Text: "_ is funny", Pick: 1}, {Text: "_ and _", Pick: 2}},
	}
}

func TestGameFlow(t *testing.T) {
	m := NewManager(testDeck())

	if _, err := m.NewGame("g", "chan", "owner"); err != nil {
		t.Fatal(err)
	}
	lobby, err := m.NewGame("g", "chan", "owner")
	if err != ErrChannelBusy {
		t.Fatalf("expected ErrChannelBusy, got %v (%+v)", err, lobby)
	}

	var gameID string
	for id := range m.byID {
		gameID = id
	}

	if _, err := m.Join(gameID, "p2"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Join(gameID, "p3"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Join(gameID, "p2"); err != nil {
		t.Fatalf("rejoin should be a no-op: %v", err)
	}

	if _, err := m.Start(gameID, "p2"); err != ErrNotOwner {
		t.Fatalf("non-owner start: %v", err)
	}
	board, err := m.Start(gameID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if board.Round != 1 || board.Phase != PhasePlaying {
		t.Fatalf("bad board after start: %+v", board)
	}

	judge := board.JudgeID
	var players []string
	for _, p := range board.Players {
		players = append(players, p.ID)
	}

	if _, err := m.PlayContext(gameID, judge); err != ErrYoureJudge {
		t.Fatalf("judge should not play: %v", err)
	}

	for _, p := range players {
		if p == judge {
			continue
		}
		ctx, err := m.PlayContext(gameID, p)
		if err != nil {
			t.Fatalf("PlayContext(%s): %v", p, err)
		}
		if len(ctx.Hand) != HandSize {
			t.Fatalf("hand size = %d", len(ctx.Hand))
		}
		idx := make([]int, ctx.Pick)
		for i := range idx {
			idx[i] = i
		}
		if _, _, err := m.SubmitPlay(gameID, p, idx); err != nil {
			t.Fatalf("SubmitPlay(%s): %v", p, err)
		}
	}

	black, cands, err := m.JudgeContext(gameID, judge)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != len(players)-1 {
		t.Fatalf("got %d candidates, want %d", len(cands), len(players)-1)
	}
	_ = black

	winner := cands[0].PlayerID
	board, err = m.SubmitJudge(gameID, judge, winner)
	if err != nil {
		t.Fatal(err)
	}
	if board.Round != 2 || board.JudgeID == judge {
		t.Fatalf("round did not advance / judge did not rotate: %+v", board)
	}

	res, err := m.End("chan", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.WinnerIDs) != 1 || res.WinnerIDs[0] != winner {
		t.Fatalf("winner = %v, want %s", res.WinnerIDs, winner)
	}
	if _, ok := m.byPlayer["owner"]; ok {
		t.Fatal("players not released after End")
	}
}

func TestDuplicateCardRejected(t *testing.T) {
	m := NewManager(testDeck())
	m.NewGame("g", "chan", "owner")
	var gameID string
	for id := range m.byID {
		gameID = id
	}
	m.Join(gameID, "p2")
	m.Join(gameID, "p3")
	board, _ := m.Start(gameID, "owner")

	// force a 2-pick prompt by reaching in
	g := m.byID[gameID]
	g.black = BlackCard{Text: "_ and _", Pick: 2}

	var nonJudge string
	for _, p := range board.Players {
		if !p.IsJudge {
			nonJudge = p.ID
			break
		}
	}
	if _, _, err := m.SubmitPlay(gameID, nonJudge, []int{0, 0}); err != ErrDuplicateCard {
		t.Fatalf("expected ErrDuplicateCard, got %v", err)
	}
}

func TestOneGamePerUser(t *testing.T) {
	m := NewManager(testDeck())
	m.NewGame("g", "chanA", "owner")
	var gameA string
	for id := range m.byID {
		gameA = id
	}
	m.Join(gameA, "shared")

	if _, err := m.NewGame("g", "chanB", "owner2"); err != nil {
		t.Fatal(err)
	}
	var gameB string
	for id := range m.byID {
		if id != gameA {
			gameB = id
		}
	}
	if _, err := m.Join(gameB, "shared"); err != ErrAlreadyInGame {
		t.Fatalf("expected ErrAlreadyInGame, got %v", err)
	}
}
