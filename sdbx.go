package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/bwmarrin/discordgo"
	"github.com/xmppo/go-xmpp"
)

type Config struct {
	Discord struct {
		Token   string
		Server  string
		Channel string
	}
	XMPP struct {
		User     string
		Password string
		Server   string
		Room     string
		Nick     string
	}
}

func main() {
	var cfg Config
	if _, err := toml.DecodeFile("config.toml", &cfg); err != nil {
		fmt.Println("ERROR:", err)
		return
	}

	options := xmpp.Options{
		Host:          cfg.XMPP.Server,
		User:          cfg.XMPP.User,
		Password:      cfg.XMPP.Password,
		NoTLS:         true,
		StartTLS:      true, // explicitly enable STARTTLS
		Debug:         true,
		Session:       true,
		Status:        "chat",
		StatusMessage: "sp's discord to xmpp bridge",
	}

	xmppbot, err := options.NewClient()

	if err != nil {
		fmt.Println("ERROR:", err)
	}
	xmppbot.JoinMUCNoHistory(cfg.XMPP.Room, cfg.XMPP.Nick)

	discbot, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		fmt.Println("ERROR:", err)
	}

	discbot.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		if m.GuildID != cfg.Discord.Server {
			return
		}
		if m.ChannelID != cfg.Discord.Channel {
			return
		}
		var msg string
		if m.MessageReference == nil {
			msg = fmt.Sprintf("%s (%s): %s", m.Author.DisplayName(), m.Author.Username, m.Content)
		} else {
			reply, err := s.ChannelMessage(m.MessageReference.ChannelID, m.MessageReference.MessageID)
			if err != nil {
				fmt.Println("ERROR:", err)
				return
			}
			msg = fmt.Sprintf("> %s (%s): %s\n%s (%s): %s", reply.Author.DisplayName(), reply.Author.Username, reply.Content, m.Author.DisplayName(), m.Author.Username, m.Content)
		}
		_, err = xmppbot.Send(xmpp.Chat{Remote: cfg.XMPP.Room, Type: "groupchat", Text: msg})
		if err != nil {
			fmt.Println("ERROR:", err)
		}
	})

	go func() {
		for {
			chat, err := xmppbot.Recv()
			if err != nil {
				fmt.Println("ERROR:", err)
				continue
			}

			switch v := chat.(type) {
			case xmpp.Chat:
				if v.Type != "groupchat" {
					continue
				}
				parts := strings.SplitN(v.Remote, "/", 2)
				var nick string
				if len(parts) == 2 {
					nick = parts[1] // the nickname part
				} else {
					nick = v.Remote
				}

				if nick == cfg.XMPP.Nick || nick == cfg.XMPP.User {
					continue
				}

				var msg string
				if len(v.Text) > 0 && v.Text[0] == '>' {
					lines := strings.Split(v.Text, "\n")
					quoteLines := []string{}
					restLines := []string{}

					foundRest := false
					for _, line := range lines {
						if strings.HasPrefix(line, ">") && !foundRest {
							trimmed := strings.TrimSpace(strings.TrimPrefix(line, ">"))
							quoteLines = append(quoteLines, trimmed)
						} else {
							foundRest = true
							restLines = append(restLines, line)
						}
					}

					replymsg := strings.Join(quoteLines, "\n")
					rest := strings.TrimSpace(strings.Join(restLines, "\n"))

					msg = fmt.Sprintf("> %s\n%s: %s", replymsg, nick, rest)
				} else {
					msg = fmt.Sprintf("%s: %s", nick, v.Text)
				}

				_, err := discbot.ChannelMessageSend(cfg.Discord.Channel, msg)
				if err != nil {
					fmt.Println("ERROR:", err)
				}

			case xmpp.Presence:
			}
		}
	}()

	err = discbot.Open()
	if err != nil {
		log.Fatal("ERROR:", err)
	}
	defer discbot.Close()

	log.Println("the bridge is running, hit ctrl c to exit")
	select {}
}
