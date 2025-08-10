package main

import (
	"fmt"
	"log"

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
		NoTLS:         false,
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
			channel, err := s.Channel(m.ChannelID)
			if err != nil {
				fmt.Println("ERROR:", err)
				return
			}
			msg = fmt.Sprintf("%s > %s: %s", channel.Name, m.Author.Username, m.Content)
		} else {
			channelID := m.MessageReference.ChannelID
			messageID := m.MessageReference.MessageID

			reply, err := s.ChannelMessage(channelID, messageID)

			if err != nil {
				fmt.Println("ERROR:", err)
			}

			rchannel, err := s.Channel(reply.ChannelID)

			if err != nil {
				fmt.Println("ERROR:", err)
				return
			}

			channel, err := s.Channel(m.ChannelID)
			if err != nil {
				fmt.Println("ERROR:", err)
				return
			}

			msg = fmt.Sprintf(">>> %s > %s: %s\n(Reply) %s > %s: %s", rchannel.Name, reply.Author.Username, reply.Content, channel.Name, m.Author.Username, m.Content)
		}
		_, err := xmppbot.Send(xmpp.Chat{Remote: cfg.XMPP.Room, Type: "groupchat", Text: msg})
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
				if v.Remote == cfg.XMPP.User || v.Type != "groupchat" {
					continue
				}

				var msg string
				msg = fmt.Sprintf("%s: %s", v.Remote, v.Text)

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
