package internal

import (
	"encoding/json"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"sync"
)

type BlackCard struct {
	Text string `json:"text"`
	Pick int    `json:"pick"`
}

type rawCards struct {
	White []string    `json:"white"`
	Black []BlackCard `json:"black"`
}

type Deck struct {
	mu         sync.Mutex
	white      []string
	black      []BlackCard
	customPath string
}

var underscoreRun = regexp.MustCompile(`_+`)

func CountPicks(text string) int {
	n := len(underscoreRun.FindAllString(text, -1))
	if n < 1 {
		return 1
	}
	return n
}

func LoadDeck(basePath, customPath string) (*Deck, error) {
	base, err := readCards(basePath)
	if err != nil {
		return nil, err
	}

	d := &Deck{
		white:      append([]string{}, base.White...),
		black:      append([]BlackCard{}, base.Black...),
		customPath: customPath,
	}

	if custom, err := readCards(customPath); err == nil {
		d.white = append(d.white, custom.White...)
		d.black = append(d.black, custom.Black...)
	}

	return d, nil
}

func readCards(path string) (*rawCards, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cards := &rawCards{}
	if err := json.Unmarshal(file, cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func (d *Deck) AddCard(cardType, text string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	custom, err := readCards(d.customPath)
	if err != nil {
		// A missing or empty file is fine; start fresh.
		custom = &rawCards{}
	}

	pick := 1
	if strings.EqualFold(cardType, "black") {
		pick = CountPicks(text)
		card := BlackCard{Text: text, Pick: pick}
		d.black = append(d.black, card)
		custom.Black = append(custom.Black, card)
	} else {
		d.white = append(d.white, text)
		custom.White = append(custom.White, text)
	}

	out, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		return pick, err
	}
	if err := os.WriteFile(d.customPath, out, 0o644); err != nil {
		return pick, err
	}
	return pick, nil
}

func (d *Deck) CustomCounts() (white, black int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	custom, err := readCards(d.customPath)
	if err != nil {
		return 0, 0
	}
	return len(custom.White), len(custom.Black)
}

func (d *Deck) ShuffledWhite() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := append([]string{}, d.white...)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func (d *Deck) ShuffledBlack() []BlackCard {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := append([]BlackCard{}, d.black...)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
