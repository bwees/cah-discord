const Filter = require('./badwords/badwords');
const Discord = require('discord.js');
const client = new Discord.Client({ intents: [Discord.Intents.FLAGS.GUILDS, Discord.Intents.FLAGS.GUILD_MESSAGES, Discord.Intents.FLAGS.GUILD_MESSAGE_REACTIONS] });
const fs = require('fs');

var context = {
    cards: {
        white: [],
        black: []
    },
    game: null
}

var filter = new Filter()

const MIN_PLAYERS = 2

const selectionEmojis = ['🇦', '🇧', '🇨', '🇩', '🇪', '🇫', '🇬', '🇭', '🇮', '🇯', '🇰', '🇱', '🇲', '🇳', '🇴', '🇵', '🇶', '🇷', '🇸', '🇹', '🇺', '🇻', '🇼', '🇽', '🇾', '🇿']

const roundEmbedTemplate = {
    "title": "Round 1",
    "description": "",
    "color": 16777215,
    "thumbnail": {
        "url": "https://cdn.discordapp.com/attachments/822103669007712309/858710210330755122/cah.png"
    },

    "fields": [
        // {
        //     "name": "A. Brandon",
        //     "value": ":white_medium_square: or :hourglass:"
        // },
    ],
    "footer": {
        "text": "Go to your Direct Messages to select **** white card."
    }
}

const cardSelectEmbedTemplate = {
    "title": "Round 1",
    "description": "",
    "color": 16777215,
    "thumbnail": {
        "url": "https://cdn.discordapp.com/attachments/822103669007712309/858710210330755122/cah.png"
    },
    "fields": [
        // {
        //     "name": "A. Donald Trump",
        //     "value": ":white_square_button: "
        // }
    ],
    "footer": {
        "text": "Select 1 Card"
    }
}

function loadCards(clean = false) {
    let cardData = fs.readFileSync('cards/cards.json');
    let cards = JSON.parse(cardData);

    var c = {
        white: [],
        black: []
    }

    if (clean) {    
        cards.white.forEach(card => {
            c.white.push(filter.clean(card))
        })
        cards.black.forEach(card => {
            c.black.push({
                    text: filter.clean(card.text),
                    pick: card.pick
            })
        })
    } else {
        c.white = cards.white
        c.black = cards.black
    }

    // CUSTOM CARDS
    let customData = fs.readFileSync('cards/custom_cards.json');
    let customCards = JSON.parse(customData);

    customCards.white.forEach(card => {
        c.white.push(clean ? filter.clean(card) : card)
    })
    customCards.black.forEach(card => {
        c.black.push({
                text: clean ? filter.clean(card.text) : card.text,
                pick: card.pick
        })
    })

    return c
}

function getRandomAndRemove(arr, n) {
    var result = new Array(n),
        len = arr.length,
        taken = new Array(len);
    if (n > len)
        throw new RangeError("getRandom: more elements taken than available");
    while (n--) {
        var x = Math.floor(Math.random() * len);
        result[n] = arr[x in taken ? taken[x] : x];
        taken[x] = --len in taken ? taken[len] : len;
    }

    arr = arr.filter(function (el) {
        return result.indexOf(el) < 0;
    });

    return result
}

function nextChar(c) {
    var i = (parseInt(c, 36) + 1) % 36;
    return (!i * 10 + i).toString(36);
}

function resetPlayerForRound(player) {
    // Remove selected Cards
    player.selection.forEach(sel => {
        player.hand.splice(player.hand.indexOf(sel), 1)
    })


    while (player.hand.length < 10) {
        player.hand.push(getRandomAndRemove(context.game.whiteCards, 1))
    }

    player.isJudge = false
    player.selection = []
    player.roundWinner = false
}

async function sendPlayerEmbed(player) {
    if (player.isJudge) {
        var sent = await player.leaveMsg.channel.send("You are the judge! :judge: Please wait for the other players to make a selection.")
        
        player.selectEmbedMessage = sent
        player.selectEmbedContent = "judge"
    }

    else {
        // Create Embed
        var selectEmbed = JSON.parse(JSON.stringify(cardSelectEmbedTemplate))
        selectEmbed.title = "Round " + context.game.currentRound
        selectEmbed.description = "`" + context.game.currentBlack.text + "`"
        selectEmbed.footer.text = `Select ${context.game.currentBlack.pick} white card${context.game.currentBlack.pick > 1 ? "s" : ""}.`
    
        var currentLetter = "A"
    
        selectEmbed.fields = []
        
        for (var i=0; i<player.hand.length; i++) {
            let card = player.hand[i]
    
            selectEmbed.fields.push({
                name: currentLetter + ". " + card,
                value: ":black_square_button:"
            })
            
            currentLetter = nextChar(currentLetter).toUpperCase()
            
        }
        
        player.remainingSelections = context.game.currentBlack.pick
        
        var sent = await player.leaveMsg.channel.send({embed: selectEmbed})
        player.selectEmbedMessage = sent
        player.selectEmbedContent = selectEmbed
        sendChooseReactions(sent, 10)
    }

}

async function sendChooseReactions(msg, num)
{
    for (var i=0; i < num; i++) {
        await msg.react(selectionEmojis[i])
    }
}

function getJudge() {
    return context.game.players.filter(p => { return p.isJudge })[0]
}

function showNextRound(winner = null) {
    if (!context.game) return
    
    context.game.players.forEach(player => {
        if (player.points == 10) {
            context.game.joinMsg.channel.send(player.user.username + " is the winner!")
            context.game = null
            return
        }
    })

    context.game.currentRound++

    // Get Black Card
    let blackCard = getRandomAndRemove(context.game.blackCards, 1)[0]
    context.game.currentBlack = blackCard
    context.game.players.forEach(p => resetPlayerForRound(p)) // Clear all judges
    

    
    context.game.judgeIndex = Math.floor(Math.random() * context.game.players.length)
    
    
    context.game.players[context.game.judgeIndex].isJudge = true
    
    // Create Embed
    var roundEmbed = JSON.parse(JSON.stringify(roundEmbedTemplate))
    roundEmbed.title = "Round " + context.game.currentRound
    roundEmbed.description = "`" + blackCard.text + "`"
    roundEmbed.footer.text = `Go to your Direct Messages to select ${blackCard.pick} white card${blackCard.pick > 1 ? "s" : ""}.`

    context.game.players.forEach(async function(player) {
        roundEmbed.fields.push({
            name: player.user.username + (player.isJudge ? "(Judge)" : "(" + player.points + " Points)"),
            value: (player.isJudge ? ":judge:" : ":hourglass:")
        })

        await sendPlayerEmbed(player)
    });


    context.game.joinMsg.channel.send({ embed: roundEmbed })
        .then(sent => {
            context.game.roundEmbedMessage = sent
            context.game.roundEmbedContent = roundEmbed
        })
}

function sendJudgingMessage() {

    var judge = getJudge()

    // Create Embed
    var selectEmbed = JSON.parse(JSON.stringify(cardSelectEmbedTemplate))
    selectEmbed.title = "Judging Round " + context.game.currentRound 
    selectEmbed.description = "`" + context.game.currentBlack.text + "`"
    selectEmbed.footer.text = `Select ${context.game.currentBlack.pick} white card${context.game.currentBlack.pick > 1 ? "s" : ""}.`

    var currentLetter = "A"
    context.game.players.forEach(player => {
        if (!player.isJudge) 
        {
            var selectedString = player.selection.join(', ')
    
            selectEmbed.fields.push({
                name: currentLetter + ". " + selectedString,
                value: ":judge:"
            })
            currentLetter = nextChar(currentLetter).toUpperCase()
        }
    })

    judge.selectEmbedMessage.channel.send({embed: selectEmbed})
        .then(sent => {
            judge.selectEmbedContent = selectEmbed
            judge.selectEmbedMessage = sent
            sendChooseReactions(sent, context.game.players.length-1)
        })
}

function updateRoundEmbed(revealCards = false) {
    var roundEmbed = context.game.roundEmbedContent

    roundEmbed.fields = []
    var readyForJudging = true

    context.game.players.forEach(player => {
        if (player.remainingSelections != 0 && !player.isJudge)
            readyForJudging = false

        roundEmbed.fields.push({
            name: (player.roundWinner ? ":star: " : " ") + player.user.username  + (player.isJudge ? " (Judge)" : " (" + player.points + " Points)"),
            value: (player.isJudge ? ":judge:" : (player.remainingSelections == 0 ? (revealCards ? player.selection.join(', ') : ":white_medium_square:") : ":hourglass:"))
        })
    });

    context.game.roundEmbedMessage.edit({ embed: roundEmbed })
    context.game.roundEmbedContent = roundEmbed

    if (readyForJudging && getJudge().selectEmbedContent == "judge")
    {
        sendJudgingMessage()
        getJudge().selectEmbedContent = "sending" // Prevent infinite Loop
    }
}

client.on('ready', () => {

    console.log("Ready!")
});

client.on("message", msg => {
    if (msg.author.bot) return

    if (msg.content.startsWith(".cah")) {
        let action = msg.content.split(" ")[1].toLowerCase()

        if (action == "create" && msg.guild != null) {
            if (context.game) {
                if (context.game.running)
                    msg.channel.send("A game is currently in progress. Please wait for it to end before joining.")
                else
                    msg.channel.send("A game is currently created, join now before it starts!")
            } else {
                msg.channel.send("New " + (msg.content.includes("clean") ? "clean " : "") + "game created! Please click the :thumbsup: to join! Enter `.cah start` to start the game.")
                    .then(sent => {
                        sent.react("👍")

                        let cards = loadCards(msg.content.includes("clean"))

                        context.game = {
                            running: false,
                            players: [],
                            joinMsg: sent,
                            whiteCards: cards.white,
                            blackCards: cards.black,
                            roundEmbed: null,
                            currentBlack: null,
                            currentRound: 0
                        }
                    });
            }
        }
        if (action == "start" && msg.guild != null) {
            if (context.game && !context.game.running) {
                if (context.game.players.length < MIN_PLAYERS) {
                    context.game.joinMsg.channel.send("There are not enough players to start the game!")
                } else {
                    context.game.running = true
                    showNextRound()
                }
            }
        }

        if (action == "end" && msg.guild != null) {
            context.game = null
            msg.channel.send("Game has been stopped and deleted.")
        }

        if (action == "custom") {
            if (msg.content.split(" ")[2].toLowerCase() == "black") {
                var newCard = msg.content.toLowerCase().slice(msg.content.toLowerCase().indexOf("black") +"black".length).trim();
                console.log(newCard)
                var pick = ((newCard.split("_").length - 1) == 0 ? 1 : newCard.split("_").length - 1)
                
                fs.readFile('cards/custom_cards.json', 'utf8', (err, data) => {
                    if (err){
                        console.log(err)
                        msg.channel.send("Error making card. Contact @bwees#3898 if this problem persists")
                    } else {
                        obj = JSON.parse(data)
                        obj.black.push({text: newCard, pick:pick})
                        json = JSON.stringify(obj)
                        fs.writeFile('cards/custom_cards.json', json, 'utf8', (err, data) => {
                            if (err) {
                                msg.channel.send("Error making card. Contact @bwees#3898 if this problem persists")
                                console.log(err)
                            } else {
                                msg.channel.send("Successfully made black card with " + pick + " answers: `"+newCard+"`")
                            }
                        })
                    }
                });
            } else if (msg.content.split(" ")[2].toLowerCase() == "white") {
                var newCard = msg.content.toLowerCase().slice(msg.content.toLowerCase().indexOf("white") +"white".length).trim();
                
                fs.readFile('cards/custom_cards.json', 'utf8', (err, data) => {
                    if (err){
                        console.log(err)
                        msg.channel.send("Error making card. Contact @bwees#3898 if this problem persists")
                    } else {
                        obj = JSON.parse(data)
                        obj.white.push(newCard)
                        json = JSON.stringify(obj)
                        fs.writeFile('cards/custom_cards.json', json, 'utf8', (err, data) => {
                            if (err)
                            {
                                msg.channel.send("Error making card. Contact @bwees#3898 if this problem persists")
                                console.log(err)
                            } else {
                                msg.channel.send("Successfully made white card: `"+newCard+"`")
                            }
                        })
                    }
                });
            } else if (msg.content.split(" ")[2].toLowerCase() == "list") {
                // TODO: Add listing of custom cards
            }
        } else if (action == "help") {
            const embed = {
                "title": "Cards Against Humanity Help",
                "color": 16777215,
                "thumbnail": {
                  "url": "https://cdn.discordapp.com/attachments/822103669007712309/858710210330755122/cah.png"
                },
                "fields": [
                  {
                    "name": "`.cah create`",
                    "value": "Creates a new game if one is not already started or in progress"
                  },
                  {
                    "name": "`.cah start`",
                    "value": "Starts a game if it has been created and is not in progress"
                  },
                  {
                    "name": "`.cah end`",
                    "value": "If a game is in progress or has been created, this will stop the game and delete any progress"
                  },
                  {
                    "name": "`.cah custom [black|white] <card text>`",
                    "value": "Creates a new custom card"
                  }
                ]
              };
              msg.channel.send({ embed: embed });
        }
    }
})

client.on("messageReactionAdd", (reaction, user) => {
    if (user.bot) return;
    if (!context.game) return;

    // Player Join Reactions
    if (reaction.message.id == context.game.joinMsg.id && reaction._emoji.name == "👍") {
        client.users.fetch(user.id, false).then((u) => {
            u.send("Welcome to the game! If you would like to leave the game, please click the ❌.")
                .then(sent => {
                    sent.react("❌")

                    let playerHand = getRandomAndRemove(context.game.whiteCards, 1)

                    context.game.players.push({
                        user: u,
                        leaveMsg: sent,
                        hand: playerHand,
                        points: 0,
                        isJudge: false,
                        selection: []
                    })
                    return
                })
        });
    }

    // Player Leave Reactions
    context.game.players.forEach(player => {
        if (reaction.message.id == player.leaveMsg.id && reaction._emoji.name == "❌") {

            const userReactions = context.game.joinMsg.reactions.cache.filter(reaction => reaction.users.cache.has(player.user.id));
            try {
                for (const reaction of userReactions.values()) {
                    reaction.users.remove(player.user.id);
                }
            } catch (error) {
                console.error('Failed to remove reactions.');
            }

            user.send("You have been removed from the game. To re-join, go back to the original invite message and click the :thumbsup: !")
            return
        }
    })

    if (selectionEmojis.indexOf(reaction._emoji.name) == -1) return

    // Player Card Choice Reactions
    context.game.players.forEach(player => {
        
        if (!player.isJudge && reaction.message.id == player.selectEmbedMessage.id) {
            if (player.remainingSelections > 0)
            {
                var cardIndex = selectionEmojis.indexOf(reaction._emoji.name)
                if (cardIndex == -1) return;
                
                player.remainingSelections--;
                player.selection.push(player.hand[cardIndex])
                

                if (player.remainingSelections == 0)
                {
                    updateRoundEmbed()
                }
            }
            return
        } 
        // Player Judging Choice
        else if (player.isJudge && reaction.message.id == player.selectEmbedMessage.id) {
            var cardIndex = selectionEmojis.indexOf(reaction._emoji.name)
            var originalIndex = selectionEmojis.indexOf(reaction._emoji.name)
            if (cardIndex == -1) return;
            
            if (cardIndex >= context.game.players.indexOf(getJudge())) // Offset Index to omit Judge     
                cardIndex++
            
            context.game.players[cardIndex].points++;
            context.game.players[cardIndex].roundWinner = true
            updateRoundEmbed(true)

            context.game.players[originalIndex].isJudge = true
            player.isJudge = false


            setTimeout(showNextRound, 10000)
        }
    })
})

client.login(process.env.TOKEN)

// Graceful shutdown on termination signals (e.g. `docker stop` sends SIGTERM)
function shutdown(signal) {
    console.log(`Received ${signal}, shutting down gracefully...`)
    client.destroy()
    process.exit(0)
}

process.on('SIGTERM', () => shutdown('SIGTERM'))
process.on('SIGINT', () => shutdown('SIGINT'))
