package internal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	ActionJoin       = "join"
	ActionLeave      = "leave"
	ActionStart      = "start"
	ActionPlay       = "play"
	ActionJudge      = "judge"
	ActionPlayModal  = "playmodal"
	ActionJudgeModal = "judgemodal"
	ActionAddModal   = "addmodal"

	cardSelectPrefix = "card" // card:<blankIndex>
	winnerSelectID   = "winner"
	addTypeSelectID  = "cardtype"
	addTextInputID   = "text"

	accentColor = 0x2B2D31 // near-black, matching CAH's black cards
)

func ParseCustomID(customID string) (action, gameID string) {
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func playersList(ids []string) string {
	mentions := make([]string, len(ids))
	for i, id := range ids {
		mentions[i] = "<@" + id + ">"
	}
	return strings.Join(mentions, "\n")
}

func LobbyComponents(v LobbyView) []discordgo.MessageComponent {
	body := fmt.Sprintf("## 🃏 Cards Against Humanity\nHosted by <@%s>", v.OwnerID)
	players := fmt.Sprintf("**Players (%d):**\n%s", len(v.Players), playersList(v.Players))
	hint := fmt.Sprintf("Need at least %d players to start.", MinPlayers)
	if v.CanStart {
		hint = "Ready when you are — the host can start the game."
	}

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: intPtr(accentColor),
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: body},
				discordgo.TextDisplay{Content: players},
				discordgo.TextDisplay{Content: hint},
				discordgo.Separator{},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.Button{Label: "Join", Style: discordgo.PrimaryButton, CustomID: ActionJoin + ":" + v.GameID},
					discordgo.Button{Label: "Leave", Style: discordgo.SecondaryButton, CustomID: ActionLeave + ":" + v.GameID},
					discordgo.Button{Label: "Start Game", Style: discordgo.SuccessButton, CustomID: ActionStart + ":" + v.GameID, Disabled: !v.CanStart},
				}},
			},
		},
	}
}

func scoreboard(v BoardView) string {
	var b strings.Builder
	b.WriteString("**Scoreboard**\n")
	for _, p := range v.Players {
		marker := "•"
		if p.IsJudge {
			marker = "👑"
		} else if v.Phase == PhasePlaying && p.Submitted {
			marker = "✅"
		}
		fmt.Fprintf(&b, "%s <@%s> — **%d**\n", marker, p.ID, p.Score)
	}
	return strings.TrimRight(b.String(), "\n")
}

func statusLine(v BoardView) string {
	switch v.Phase {
	case PhaseJudging:
		return fmt.Sprintf("✋ All answers are in! Judge <@%s>, make your pick.", v.JudgeID)
	default:
		in := 0
		for _, p := range v.Players {
			if p.Submitted {
				in++
			}
		}
		return fmt.Sprintf("✍️ Waiting on players… (%d/%d in). Judge: <@%s>", in, len(v.Players)-1, v.JudgeID)
	}
}

func BoardComponents(v BoardView) []discordgo.MessageComponent {
	comps := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: fmt.Sprintf("## 🃏 Cards Against Humanity — Round %d", v.Round)},
	}

	if v.LastWinnerID != "" {
		comps = append(comps, discordgo.TextDisplay{
			Content: fmt.Sprintf("🏆 <@%s> won the last round:\n> %s", v.LastWinnerID, v.LastFilled),
		}, discordgo.Separator{})
	}

	comps = append(comps,
		discordgo.TextDisplay{Content: fmt.Sprintf("**Black Card** — pick %d\n> %s", v.Pick, v.BlackText)},
		discordgo.TextDisplay{Content: scoreboard(v)},
		discordgo.TextDisplay{Content: statusLine(v)},
		discordgo.Separator{},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label: "Play Cards", Style: discordgo.PrimaryButton,
				CustomID: ActionPlay + ":" + v.GameID, Disabled: v.Phase != PhasePlaying,
			},
			discordgo.Button{
				Label: "Judge", Style: discordgo.SuccessButton,
				CustomID: ActionJudge + ":" + v.GameID, Disabled: v.Phase != PhaseJudging,
			},
		}},
	)

	return []discordgo.MessageComponent{
		discordgo.Container{AccentColor: intPtr(accentColor), Components: comps},
	}
}

func PlayModal(gameID string, ctx PlayContext) *discordgo.InteractionResponse {
	var hand strings.Builder
	hand.WriteString(fmt.Sprintf("**Black Card** — pick %d\n> %s\n\n**Your hand:**\n", ctx.Pick, ctx.BlackText))
	for _, c := range ctx.Hand {
		fmt.Fprintf(&hand, "• %s\n", c)
	}

	options := make([]discordgo.SelectMenuOption, len(ctx.Hand))
	for i, c := range ctx.Hand {
		options[i] = discordgo.SelectMenuOption{
			Label: truncate(c, 100),
			Value: strconv.Itoa(i),
		}
	}

	comps := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: strings.TrimRight(hand.String(), "\n")},
	}
	for b := 0; b < ctx.Pick; b++ {
		label := "Your card"
		if ctx.Pick > 1 {
			label = fmt.Sprintf("Card for blank %d", b+1)
		}
		comps = append(comps, discordgo.Label{
			Label: label,
			Component: discordgo.SelectMenu{
				MenuType:    discordgo.StringSelectMenu,
				CustomID:    fmt.Sprintf("%s:%d", cardSelectPrefix, b),
				Placeholder: "Choose a card…",
				MinValues:   intPtr(1),
				MaxValues:   1,
				Required:    boolPtr(true),
				Options:     options,
			},
		})
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   ActionPlayModal + ":" + gameID,
			Title:      "Play Your Cards",
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: comps,
		},
	}
}

func JudgeModal(gameID, blackText string, candidates []Candidate) *discordgo.InteractionResponse {
	var list strings.Builder
	list.WriteString(fmt.Sprintf("**Black Card**\n> %s\n\n**The answers:**\n", blackText))
	options := make([]discordgo.SelectMenuOption, len(candidates))
	for i, c := range candidates {
		fmt.Fprintf(&list, "**%d.** %s\n", i+1, c.Filled)
		options[i] = discordgo.SelectMenuOption{
			Label: fmt.Sprintf("Answer %d", i+1),
			Value: c.PlayerID,
		}
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: ActionJudgeModal + ":" + gameID,
			Title:    "Pick the Winner",
			Flags:    discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: strings.TrimRight(list.String(), "\n")},
				discordgo.Label{
					Label: "Funniest answer",
					Component: discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    winnerSelectID,
						Placeholder: "Choose the winning answer…",
						MinValues:   intPtr(1),
						MaxValues:   1,
						Required:    boolPtr(true),
						Options:     options,
					},
				},
			},
		},
	}
}

func ResultsComponents(v ResultsView) []discordgo.MessageComponent {
	var winnerLine string
	switch len(v.WinnerIDs) {
	case 0:
		winnerLine = "No winners — the game ended early."
	case 1:
		winnerLine = fmt.Sprintf("**Winner:** <@%s> 🎉", v.WinnerIDs[0])
	default:
		mentions := make([]string, len(v.WinnerIDs))
		for i, id := range v.WinnerIDs {
			mentions[i] = "<@" + id + ">"
		}
		winnerLine = "**It's a tie!** " + strings.Join(mentions, ", ") + " 🎉"
	}

	var scores strings.Builder
	scores.WriteString("**Final Scores**\n")
	for i, p := range v.Standings {
		fmt.Fprintf(&scores, "%d. <@%s> — **%d**\n", i+1, p.ID, p.Score)
	}

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: intPtr(accentColor),
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: fmt.Sprintf("## 🏁 Game Over — %d rounds played", v.Rounds)},
				discordgo.TextDisplay{Content: winnerLine},
				discordgo.Separator{},
				discordgo.TextDisplay{Content: strings.TrimRight(scores.String(), "\n")},
			},
		},
	}
}

func AddCardModal() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: ActionAddModal,
			Title:    "Add a Custom Card",
			Flags:    discordgo.MessageFlagsIsComponentsV2,
			Components: []discordgo.MessageComponent{
				discordgo.Label{
					Label:       "Card type",
					Description: "White is an answer, black is a prompt",
					Component: discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    addTypeSelectID,
						Placeholder: "Pick a deck…",
						MinValues:   intPtr(1),
						MaxValues:   1,
						Required:    boolPtr(true),
						Options: []discordgo.SelectMenuOption{
							{Label: "White card", Value: "white", Description: "An answer card", Emoji: &discordgo.ComponentEmoji{Name: "⬜"}},
							{Label: "Black card", Value: "black", Description: "A prompt — use _ for each blank", Emoji: &discordgo.ComponentEmoji{Name: "⬛"}},
						},
					},
				},
				discordgo.Label{
					Label:       "Card text",
					Description: "On black cards, use _ for each blank (__ counts as one).",
					Component: discordgo.TextInput{
						CustomID:    addTextInputID,
						Style:       discordgo.TextInputParagraph,
						Required:    boolPtr(true),
						MaxLength:   300,
						Placeholder: "e.g. Why did the _ cross the road?",
					},
				},
			},
		},
	}
}

func AddedCardComponents(authorID, cardType, text string, pick int) []discordgo.MessageComponent {
	header := fmt.Sprintf("➕ <@%s> added a **white card**", authorID)
	body := text
	if cardType == "black" {
		header = fmt.Sprintf("➕ <@%s> added a **black card** — pick %d", authorID, pick)
		body = displayBlack(text)
	}
	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: intPtr(accentColor),
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: header},
				discordgo.TextDisplay{Content: "> " + body},
			},
		},
	}
}
