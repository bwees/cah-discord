package internal

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
)

const (
	HandSize   = 10
	MinPlayers = 3 // judge plus at least two answers to choose between
)

type Phase int

const (
	PhaseLobby Phase = iota
	PhasePlaying
	PhaseJudging
	PhaseEnded
)

var (
	ErrNotFound      = errors.New("that game no longer exists")
	ErrAlreadyInGame = errors.New("you're already in a game — finish or leave it first")
	ErrChannelBusy   = errors.New("a game is already running in this channel")
	ErrNotInGame     = errors.New("you're not in this game")
	ErrNotOwner      = errors.New("only the game's host can do that")
	ErrNotEnough     = fmt.Errorf("you need at least %d players to start", MinPlayers)
	ErrWrongPhase    = errors.New("you can't do that right now")
	ErrYoureJudge    = errors.New("you're the judge this round — sit back and wait for the answers")
	ErrNotJudge      = errors.New("only this round's judge can do that")
	ErrAlreadyPlayed = errors.New("you've already played your cards this round")
	ErrDuplicateCard = errors.New("you can't play the same card twice — pick a different card for each blank")
	ErrInvalidPick   = errors.New("invalid card selection")
)

type submission struct {
	playerID string
	cards    []string
}

// Access is serialized through the owning Manager's mutex; Game has no lock.
type Game struct {
	ID        string
	GuildID   string
	ChannelID string
	OwnerID   string
	MessageID string

	players    []string
	hands      map[string][]string
	scores     map[string]int
	judgeIndex int

	black       BlackCard
	submissions map[string]*submission
	phase       Phase
	round       int

	lastWinnerID string
	lastFilled   string

	whiteDeck []string
	whiteIdx  int
	blackDeck []BlackCard
	blackIdx  int
}

func (g *Game) judgeID() string {
	if len(g.players) == 0 {
		return ""
	}
	return g.players[g.judgeIndex%len(g.players)]
}

func (g *Game) drawWhite(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if g.whiteIdx >= len(g.whiteDeck) {
			rand.Shuffle(len(g.whiteDeck), func(a, b int) {
				g.whiteDeck[a], g.whiteDeck[b] = g.whiteDeck[b], g.whiteDeck[a]
			})
			g.whiteIdx = 0
		}
		out = append(out, g.whiteDeck[g.whiteIdx])
		g.whiteIdx++
	}
	return out
}

func (g *Game) drawBlack() BlackCard {
	if g.blackIdx >= len(g.blackDeck) {
		rand.Shuffle(len(g.blackDeck), func(a, b int) {
			g.blackDeck[a], g.blackDeck[b] = g.blackDeck[b], g.blackDeck[a]
		})
		g.blackIdx = 0
	}
	c := g.blackDeck[g.blackIdx]
	g.blackIdx++
	return c
}

func (g *Game) refillHands() {
	for _, p := range g.players {
		if missing := HandSize - len(g.hands[p]); missing > 0 {
			g.hands[p] = append(g.hands[p], g.drawWhite(missing)...)
		}
	}
}

func (g *Game) startRound() {
	g.black = g.drawBlack()
	g.submissions = map[string]*submission{}
	g.refillHands()
	g.phase = PhasePlaying
}

func (g *Game) allSubmitted() bool {
	return len(g.submissions) >= len(g.players)-1
}

type PlayerView struct {
	ID        string
	Score     int
	IsJudge   bool
	Submitted bool
}

type BoardView struct {
	GameID       string
	ChannelID    string
	MessageID    string
	Round        int
	Phase        Phase
	BlackText    string
	Pick         int
	Players      []PlayerView
	JudgeID      string
	AllSubmitted bool
	LastWinnerID string
	LastFilled   string
}

func (g *Game) boardView() BoardView {
	judge := g.judgeID()
	players := make([]PlayerView, 0, len(g.players))
	for _, p := range g.players {
		_, submitted := g.submissions[p]
		players = append(players, PlayerView{
			ID:        p,
			Score:     g.scores[p],
			IsJudge:   p == judge,
			Submitted: submitted,
		})
	}
	return BoardView{
		GameID:       g.ID,
		ChannelID:    g.ChannelID,
		MessageID:    g.MessageID,
		Round:        g.round,
		Phase:        g.phase,
		BlackText:    displayBlack(g.black.Text),
		Pick:         g.black.Pick,
		Players:      players,
		JudgeID:      judge,
		AllSubmitted: g.allSubmitted(),
		LastWinnerID: g.lastWinnerID,
		LastFilled:   g.lastFilled,
	}
}

type LobbyView struct {
	GameID   string
	OwnerID  string
	Players  []string
	CanStart bool
}

func (g *Game) lobbyView() LobbyView {
	return LobbyView{
		GameID:   g.ID,
		OwnerID:  g.OwnerID,
		Players:  append([]string{}, g.players...),
		CanStart: len(g.players) >= MinPlayers,
	}
}

type Candidate struct {
	PlayerID string
	Filled   string
}

type ResultsView struct {
	Rounds    int
	Standings []PlayerView // sorted high to low
	WinnerIDs []string
}

type Manager struct {
	mu        sync.Mutex
	deck      *Deck
	byID      map[string]*Game
	byChannel map[string]*Game
	byPlayer  map[string]*Game
	nextID    int
}

func NewManager(deck *Deck) *Manager {
	return &Manager{
		deck:      deck,
		byID:      map[string]*Game{},
		byChannel: map[string]*Game{},
		byPlayer:  map[string]*Game{},
	}
}

func (m *Manager) AddCard(cardType, text string) (int, error) {
	return m.deck.AddCard(cardType, text)
}

func (m *Manager) CustomCounts() (white, black int) {
	return m.deck.CustomCounts()
}

func (m *Manager) NewGame(guildID, channelID, ownerID string) (LobbyView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byChannel[channelID]; ok {
		return LobbyView{}, ErrChannelBusy
	}
	if _, ok := m.byPlayer[ownerID]; ok {
		return LobbyView{}, ErrAlreadyInGame
	}

	m.nextID++
	g := &Game{
		ID:          strconv.Itoa(m.nextID),
		GuildID:     guildID,
		ChannelID:   channelID,
		OwnerID:     ownerID,
		players:     []string{ownerID},
		hands:       map[string][]string{ownerID: {}},
		scores:      map[string]int{ownerID: 0},
		submissions: map[string]*submission{},
		phase:       PhaseLobby,
	}
	m.byID[g.ID] = g
	m.byChannel[channelID] = g
	m.byPlayer[ownerID] = g
	return g.lobbyView(), nil
}

func (m *Manager) SetMessageID(gameID, messageID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.byID[gameID]; ok {
		g.MessageID = messageID
	}
}

func (m *Manager) Join(gameID, userID string) (LobbyView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return LobbyView{}, ErrNotFound
	}
	if g.phase != PhaseLobby {
		return LobbyView{}, ErrWrongPhase
	}
	if existing, ok := m.byPlayer[userID]; ok {
		if existing == g {
			return g.lobbyView(), nil // already joined; just refresh
		}
		return LobbyView{}, ErrAlreadyInGame
	}
	g.players = append(g.players, userID)
	g.hands[userID] = []string{}
	g.scores[userID] = 0
	m.byPlayer[userID] = g
	return g.lobbyView(), nil
}

func (m *Manager) Leave(gameID, userID string) (view LobbyView, disbanded bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return LobbyView{}, false, ErrNotFound
	}
	if g.phase != PhaseLobby {
		return LobbyView{}, false, ErrWrongPhase
	}
	if _, ok := g.scores[userID]; !ok {
		return LobbyView{}, false, ErrNotInGame
	}
	if userID == g.OwnerID {
		m.remove(g)
		return LobbyView{}, true, nil
	}
	g.removePlayer(userID)
	delete(m.byPlayer, userID)
	return g.lobbyView(), false, nil
}

func (g *Game) removePlayer(userID string) {
	for i, p := range g.players {
		if p == userID {
			g.players = append(g.players[:i], g.players[i+1:]...)
			break
		}
	}
	delete(g.hands, userID)
	delete(g.scores, userID)
	delete(g.submissions, userID)
}

func (m *Manager) Start(gameID, userID string) (BoardView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return BoardView{}, ErrNotFound
	}
	if g.phase != PhaseLobby {
		return BoardView{}, ErrWrongPhase
	}
	if userID != g.OwnerID {
		return BoardView{}, ErrNotOwner
	}
	if len(g.players) < MinPlayers {
		return BoardView{}, ErrNotEnough
	}

	g.whiteDeck = m.deck.ShuffledWhite()
	g.blackDeck = m.deck.ShuffledBlack()
	g.judgeIndex = 0
	g.round = 1
	g.startRound()
	return g.boardView(), nil
}

type PlayContext struct {
	Hand      []string
	BlackText string
	Pick      int
}

func (m *Manager) PlayContext(gameID, userID string) (PlayContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return PlayContext{}, ErrNotFound
	}
	if _, ok := g.scores[userID]; !ok {
		return PlayContext{}, ErrNotInGame
	}
	if g.phase != PhasePlaying {
		return PlayContext{}, ErrWrongPhase
	}
	if userID == g.judgeID() {
		return PlayContext{}, ErrYoureJudge
	}
	if _, ok := g.submissions[userID]; ok {
		return PlayContext{}, ErrAlreadyPlayed
	}
	return PlayContext{
		Hand:      append([]string{}, g.hands[userID]...),
		BlackText: displayBlack(g.black.Text),
		Pick:      g.black.Pick,
	}, nil
}

func (m *Manager) SubmitPlay(gameID, userID string, indices []int) (view BoardView, nowJudging bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return BoardView{}, false, ErrNotFound
	}
	if g.phase != PhasePlaying {
		return BoardView{}, false, ErrWrongPhase
	}
	if userID == g.judgeID() {
		return BoardView{}, false, ErrYoureJudge
	}
	if _, ok := g.submissions[userID]; ok {
		return BoardView{}, false, ErrAlreadyPlayed
	}

	hand := g.hands[userID]
	if len(indices) != g.black.Pick {
		return BoardView{}, false, ErrInvalidPick
	}
	seen := map[int]bool{}
	chosen := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(hand) {
			return BoardView{}, false, ErrInvalidPick
		}
		if seen[idx] {
			return BoardView{}, false, ErrDuplicateCard
		}
		seen[idx] = true
		chosen = append(chosen, hand[idx])
	}

	// Remove chosen cards from the hand (highest index first), then refill.
	idxDesc := append([]int{}, indices...)
	sort.Sort(sort.Reverse(sort.IntSlice(idxDesc)))
	for _, idx := range idxDesc {
		g.hands[userID] = append(g.hands[userID][:idx], g.hands[userID][idx+1:]...)
	}
	g.hands[userID] = append(g.hands[userID], g.drawWhite(len(indices))...)

	g.submissions[userID] = &submission{playerID: userID, cards: chosen}
	if g.allSubmitted() {
		g.phase = PhaseJudging
	}
	return g.boardView(), g.phase == PhaseJudging, nil
}

func (m *Manager) JudgeContext(gameID, userID string) (black string, candidates []Candidate, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return "", nil, ErrNotFound
	}
	if g.phase != PhaseJudging {
		return "", nil, ErrWrongPhase
	}
	if userID != g.judgeID() {
		return "", nil, ErrNotJudge
	}

	for _, p := range g.players {
		if sub, ok := g.submissions[p]; ok {
			candidates = append(candidates, Candidate{
				PlayerID: p,
				Filled:   fillBlack(g.black.Text, sub.cards),
			})
		}
	}
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	return displayBlack(g.black.Text), candidates, nil
}

func (m *Manager) SubmitJudge(gameID, userID, winnerID string) (BoardView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byID[gameID]
	if g == nil {
		return BoardView{}, ErrNotFound
	}
	if g.phase != PhaseJudging {
		return BoardView{}, ErrWrongPhase
	}
	if userID != g.judgeID() {
		return BoardView{}, ErrNotJudge
	}
	sub, ok := g.submissions[winnerID]
	if !ok {
		return BoardView{}, ErrInvalidPick
	}

	g.scores[winnerID]++
	g.lastWinnerID = winnerID
	g.lastFilled = fillBlack(g.black.Text, sub.cards)

	g.judgeIndex = (g.judgeIndex + 1) % len(g.players)
	g.round++
	g.startRound()
	return g.boardView(), nil
}

func (m *Manager) End(channelID, userID string) (ResultsView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.byChannel[channelID]
	if g == nil {
		return ResultsView{}, ErrNotFound
	}
	if userID != g.OwnerID {
		return ResultsView{}, ErrNotOwner
	}

	standings := make([]PlayerView, 0, len(g.players))
	for _, p := range g.players {
		standings = append(standings, PlayerView{ID: p, Score: g.scores[p]})
	}
	sort.SliceStable(standings, func(i, j int) bool {
		return standings[i].Score > standings[j].Score
	})

	var winners []string
	if len(standings) > 0 {
		top := standings[0].Score
		for _, s := range standings {
			if s.Score == top {
				winners = append(winners, s.ID)
			}
		}
	}

	g.phase = PhaseEnded
	m.remove(g)
	return ResultsView{Rounds: g.round, Standings: standings, WinnerIDs: winners}, nil
}

// remove unregisters a game from all lookup maps. Caller must hold m.mu.
func (m *Manager) remove(g *Game) {
	delete(m.byID, g.ID)
	if m.byChannel[g.ChannelID] == g {
		delete(m.byChannel, g.ChannelID)
	}
	for _, p := range g.players {
		if m.byPlayer[p] == g {
			delete(m.byPlayer, p)
		}
	}
}
