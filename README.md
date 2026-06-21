# Cards Against Humanity Discord Bot

A Discord bot (written in Go with [discordgo](https://github.com/bwmarrin/discordgo))
for playing Cards Against Humanity with friends on a server. Everything happens
through buttons and modals — no chat commands during play.

## Commands

| Command | What it does |
| --- | --- |
| `/cah new` | Open a lobby in the current channel. Others join with the **Join** button; the host presses **Start Game** (needs 3+ players). |
| `/cah end` | End the game (host only) and post the final scoreboard and winner. |
| `/add <card_type> <text>` | Add a custom card to the deck. `card_type` is a White/Black picker. For black cards the number of blanks is taken from the text — each run of underscores (`_` or `__`) counts as one blank; no underscores means one pick. |

## How a round works

- The board message shows the black card, the scoreboard (👑 marks the judge,
  ✅ marks who has played), and **Play Cards** / **Judge** buttons.
- **Play Cards** opens a modal listing your hand and one card-picker per blank.
  You can't play the same card for two blanks.
- Once everyone but the judge has played, the judge presses **Judge** and picks
  the winning answer from a list, with each answer filled into the black card
  (answers are **bold + underlined**, the prompt stays normal).
- The winner scores a point, the judge rotates, and the next round begins in the
  same message.

Buttons are enabled only for the relevant phase; clicking one you can't use
(e.g. the judge trying to play) returns a private error message.

## Rules / constraints

- Multiple games can run at once (one per channel).
- A user can only be in one game at a time.

## Running

```sh
go build -o cah .
./cah -token <BOT_TOKEN> -app <APPLICATION_ID> [-guild <GUILD_ID>]
```

Flags: `-token`, `-app`, `-guild` (omit for global commands), `-cleanup`
(delete commands on shutdown, default true), `-cards`, `-custom`.

### Docker

```sh
docker build -t cah-discord .
docker run cah-discord -token <BOT_TOKEN> -app <APPLICATION_ID>
```

Custom cards are written to `cards/custom_cards.json`. Mount a volume there if
you want them to survive container restarts.
