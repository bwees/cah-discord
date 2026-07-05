package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"cah-discord/internal"

	"github.com/bwmarrin/discordgo"
)

var version = "dev"

var (
	GuildID  = flag.String("guild", "", "Test guild ID (empty = global commands)")
	BotToken = flag.String("token", os.Getenv("DISCORD_TOKEN"), "Bot access token (env DISCORD_TOKEN)")
	AppID    = flag.String("app", os.Getenv("DISCORD_APP_ID"), "Application ID (env DISCORD_APP_ID)")
	Cleanup  = flag.Bool("cleanup", true, "Delete commands on shutdown")
	BaseDeck = flag.String("cards", "cards/cards.json", "Path to base card file")
	CustDeck = flag.String("custom", "cards/custom_cards.json", "Path to custom card file")
)

func main() {
	flag.Parse()

	log.Printf("cah-discord %s", version)

	if *BotToken == "" || *AppID == "" {
		log.Fatal("DISCORD_TOKEN and DISCORD_APP_ID must be set (or pass -token/-app)")
	}

	deck, err := internal.LoadDeck(*BaseDeck, *CustDeck)
	if err != nil {
		log.Fatalf("Cannot load cards: %v", err)
	}

	s, err := discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}

	bot := internal.NewBot(s, internal.NewManager(deck))

	s.AddHandler(func(_ *discordgo.Session, _ *discordgo.Ready) {
		log.Println("Bot is up!")
	})
	s.AddHandler(bot.HandleInteraction)

	if err := s.Open(); err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}
	defer s.Close()

	commands := bot.Commands()
	registered := make([]*discordgo.ApplicationCommand, 0, len(commands))
	for _, cmd := range commands {
		rcmd, err := s.ApplicationCommandCreate(*AppID, *GuildID, cmd)
		if err != nil {
			log.Fatalf("Cannot create command %q: %v", cmd.Name, err)
		}
		registered = append(registered, rcmd)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Graceful shutdown")

	if !*Cleanup {
		return
	}
	for _, cmd := range registered {
		if err := s.ApplicationCommandDelete(*AppID, *GuildID, cmd.ID); err != nil {
			log.Printf("Cannot delete command %q: %v", cmd.Name, err)
		}
	}
}
