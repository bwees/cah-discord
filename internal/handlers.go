package internal

import (
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	s *discordgo.Session
	m *Manager
}

func NewBot(s *discordgo.Session, m *Manager) *Bot {
	return &Bot{s: s, m: m}
}

func (b *Bot) Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "cah",
			Description: "Play Cards Against Humanity",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "new", Description: "Start a new game in this channel"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "end", Description: "End the game and show the results"},
			},
		},
		{
			Name:        "add",
			Description: "Add a custom card to the deck",
		},
		{
			Name:        "cards",
			Description: "Show how many custom cards have been added",
		},
	}
}

func (b *Bot) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(i)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(i)
	case discordgo.InteractionModalSubmit:
		b.handleModal(i)
	}
}

func userID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (b *Bot) handleCommand(i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	switch data.Name {
	case "cah":
		if len(data.Options) == 0 {
			return
		}
		switch data.Options[0].Name {
		case "new":
			b.handleNew(i)
		case "end":
			b.handleEnd(i)
		}
	case "add":
		b.handleAdd(i)
	case "cards":
		b.handleCards(i)
	}
}

func (b *Bot) handleNew(i *discordgo.InteractionCreate) {
	view, err := b.m.NewGame(i.GuildID, i.ChannelID, userID(i))
	if err != nil {
		b.respondError(i, err)
		return
	}
	b.respondComponents(i, LobbyComponents(view))

	msg, err := b.s.InteractionResponse(i.Interaction)
	if err != nil {
		log.Printf("fetch lobby message: %v", err)
		return
	}
	b.m.SetMessageID(view.GameID, msg.ID)
}

func (b *Bot) handleEnd(i *discordgo.InteractionCreate) {
	view, err := b.m.End(i.ChannelID, userID(i))
	if err != nil {
		b.respondError(i, err)
		return
	}
	b.respondComponents(i, ResultsComponents(view))
}

func (b *Bot) handleAdd(i *discordgo.InteractionCreate) {
	if err := b.s.InteractionRespond(i.Interaction, AddCardModal()); err != nil {
		log.Printf("add modal: %v", err)
	}
}

func (b *Bot) handleCards(i *discordgo.InteractionCreate) {
	white, black := b.m.CustomCounts()
	b.respondComponents(i, CustomCardsComponents(white, black))
}

func (b *Bot) handleComponent(i *discordgo.InteractionCreate) {
	action, gameID := ParseCustomID(i.MessageComponentData().CustomID)
	uid := userID(i)

	switch action {
	case ActionJoin:
		view, err := b.m.Join(gameID, uid)
		if err != nil {
			b.respondError(i, err)
			return
		}
		b.updateComponents(i, LobbyComponents(view))
	case ActionLeave:
		view, disbanded, err := b.m.Leave(gameID, uid)
		if err != nil {
			b.respondError(i, err)
			return
		}
		if disbanded {
			b.updateComponents(i, disbandedComponents())
			return
		}
		b.updateComponents(i, LobbyComponents(view))
	case ActionStart:
		view, err := b.m.Start(gameID, uid)
		if err != nil {
			b.respondError(i, err)
			return
		}
		view.MessageID = i.Message.ID
		view.ChannelID = i.ChannelID
		b.m.SetMessageID(gameID, i.Message.ID)
		b.updateComponents(i, BoardComponents(view))
	case ActionPlay:
		ctx, err := b.m.PlayContext(gameID, uid)
		if err != nil {
			b.respondError(i, err)
			return
		}
		if err := b.s.InteractionRespond(i.Interaction, PlayModal(gameID, ctx)); err != nil {
			log.Printf("play modal: %v", err)
		}
	case ActionJudge:
		black, candidates, err := b.m.JudgeContext(gameID, uid)
		if err != nil {
			b.respondError(i, err)
			return
		}
		if err := b.s.InteractionRespond(i.Interaction, JudgeModal(gameID, black, candidates)); err != nil {
			log.Printf("judge modal: %v", err)
		}
	}
}

func (b *Bot) handleModal(i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	action, gameID := ParseCustomID(data.CustomID)
	uid := userID(i)

	switch action {
	case ActionPlayModal:
		indices, err := playIndices(data)
		if err != nil {
			b.respondError(i, err)
			return
		}
		view, _, err := b.m.SubmitPlay(gameID, uid, indices)
		if err != nil {
			b.respondError(i, err)
			return
		}
		b.respondEphemeral(i, "Your cards are in! 🎴")
		b.editBoard(view)
	case ActionJudgeModal:
		winner, err := judgeWinner(data)
		if err != nil {
			b.respondError(i, err)
			return
		}
		view, err := b.m.SubmitJudge(gameID, uid, winner)
		if err != nil {
			b.respondError(i, err)
			return
		}
		b.respondEphemeral(i, "Winner picked! On to the next round.")
		b.editBoard(view)
	case ActionAddModal:
		cardType, text := addCardFields(data)
		text = strings.TrimSpace(text)
		if cardType == "" || text == "" {
			b.respondEphemeral(i, "Please choose a card type and enter some text.")
			return
		}
		pick, err := b.m.AddCard(cardType, text)
		if err != nil {
			log.Printf("add card: %v", err)
			b.respondEphemeral(i, "Couldn't save that card, sorry.")
			return
		}
		b.respondComponents(i, AddedCardComponents(uid, cardType, text, pick))
	}
}

func addCardFields(data discordgo.ModalSubmitInteractionData) (cardType, text string) {
	for _, c := range data.Components {
		label, ok := c.(*discordgo.Label)
		if !ok {
			continue
		}
		switch comp := label.Component.(type) {
		case *discordgo.SelectMenu:
			if comp.CustomID == addTypeSelectID && len(comp.Values) > 0 {
				cardType = comp.Values[0]
			}
		case *discordgo.TextInput:
			if comp.CustomID == addTextInputID {
				text = comp.Value
			}
		}
	}
	return
}

func playIndices(data discordgo.ModalSubmitInteractionData) ([]int, error) {
	type pair struct{ blank, idx int }
	var pairs []pair
	for _, c := range data.Components {
		label, ok := c.(*discordgo.Label)
		if !ok {
			continue
		}
		sel, ok := label.Component.(*discordgo.SelectMenu)
		if !ok || !strings.HasPrefix(sel.CustomID, cardSelectPrefix+":") || len(sel.Values) == 0 {
			continue
		}
		idx, err := strconv.Atoi(sel.Values[0])
		if err != nil {
			return nil, ErrInvalidPick
		}
		blank, _ := strconv.Atoi(strings.TrimPrefix(sel.CustomID, cardSelectPrefix+":"))
		pairs = append(pairs, pair{blank, idx})
	}
	if len(pairs) == 0 {
		return nil, ErrInvalidPick
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].blank < pairs[j].blank })
	out := make([]int, len(pairs))
	for i, p := range pairs {
		out[i] = p.idx
	}
	return out, nil
}

func judgeWinner(data discordgo.ModalSubmitInteractionData) (string, error) {
	for _, c := range data.Components {
		label, ok := c.(*discordgo.Label)
		if !ok {
			continue
		}
		if sel, ok := label.Component.(*discordgo.SelectMenu); ok && sel.CustomID == winnerSelectID && len(sel.Values) > 0 {
			return sel.Values[0], nil
		}
	}
	return "", ErrInvalidPick
}

func disbandedComponents() []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: intPtr(accentColor),
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: "🃏 The host left — this game has been disbanded."},
			},
		},
	}
}

func (b *Bot) editBoard(v BoardView) {
	if v.MessageID == "" {
		return
	}
	comps := BoardComponents(v)
	_, err := b.s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    v.ChannelID,
		ID:         v.MessageID,
		Components: &comps,
		Flags:      discordgo.MessageFlagsIsComponentsV2,
	})
	if err != nil {
		log.Printf("edit board: %v", err)
	}
}

func (b *Bot) respondComponents(i *discordgo.InteractionCreate, comps []discordgo.MessageComponent) {
	err := b.s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: comps,
		},
	})
	if err != nil {
		log.Printf("respond components: %v", err)
	}
}

func (b *Bot) updateComponents(i *discordgo.InteractionCreate, comps []discordgo.MessageComponent) {
	err := b.s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: comps,
		},
	})
	if err != nil {
		log.Printf("update components: %v", err)
	}
}

func (b *Bot) respondEphemeral(i *discordgo.InteractionCreate, msg string) {
	err := b.s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("respond ephemeral: %v", err)
	}
}

func (b *Bot) respondError(i *discordgo.InteractionCreate, err error) {
	b.respondEphemeral(i, "⚠️ "+err.Error())
}
