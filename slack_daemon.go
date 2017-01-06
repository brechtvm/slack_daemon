package main

import (
	"flag"
	"fmt"
	"github.com/nlopes/slack"
	logrus "github.com/sirupsen/logrus"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var api *slack.Client
var msgTimestamps []string
var channels []slack.Channel
var evtChannelInfo *slack.ChannelInfoEvent
var chMessages chan slack.Msg
var slackToken string
var allChannels []slack.Channel
var rtm *slack.RTM

/*
// TODO
- A bot can post multiple messages at a time
- if strings.Contains(message.Text, "@") {
*/

func main() {
	parseFlags()

	//chTopics := make(chan slack.Channel)
	chMessages = make(chan slack.Msg, 100)

	api = slack.New(slackToken)
	// If you set debugging, it will log all requests to the console
	// Useful when encountering issues
	//api.SetDebug(true)

	logger := log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)
	slack.SetLogger(logger)

	// Read & print all messages
	go readMessages()

	// Fetch all events
	fetchEvents()
}

func parseFlags() {
	var tokenFlag = flag.String("token", "", "your slack token")
	var logLevelFlag = flag.String("logLevel", "info", "logrus log level")

	flag.Parse()

	if *tokenFlag == "" {
		log.Fatal("Parameter token not given.")
	}
	slackToken = *tokenFlag

	if *logLevelFlag == "" {
		log.Fatal("")
	} else {
		logLevel, err := logrus.ParseLevel(*logLevelFlag)
		if err != nil {
			logrus.Errorf("failed parsing log level, defaulting to `info`. err - %v", err)
			logLevel = logrus.InfoLevel
		}
		logrus.SetLevel(logLevel)
	}
}

func readMessages() {
	for {
		message := <-chMessages
		if strings.Contains(message.Text, "<@") {
			// cast username
		}
		info := rtm.GetInfo()
		channelName := info.GetChannelByID(message.Channel).Name
		output := fmt.Sprintf("%s [%s] %s: %s \r\n", channelName, timeconvert(message.Timestamp), message.Username, message.Text)
		write2file(fmt.Sprintf("%s.log", channelName), output)
		write2file("debug.log", fmt.Sprintf("%v\n", message))
	}
}

func write2file(filename string, message string) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		log.Fatal(message, err)
	}
	defer file.Close()

	_, err = file.WriteString(message)

	if err != nil {
		log.Fatal(message, err)
	}
}

// Control center
func fetchEvents() {
	api.SetDebug(false)
	rtm = api.NewRTM()
	go rtm.ManageConnection()

	excludeArchivedChannels := true
	allChannels, _ = rtm.GetChannels(excludeArchivedChannels)

	log.Println("Watching:")
	for _, channel := range allChannels {
		log.Printf("Channel #%s\n", channel.Name)
	}

	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.ConnectedEvent:
			case *slack.MessageEvent:
				info := rtm.GetInfo()

				if ev.DeletedTimestamp != "" {
					fmt.Printf("Message deleted: [%v]\n", ev.Msg)
					continue
				}
				if ev.Edited != nil && ev.Edited.Timestamp != "" {
					fmt.Printf("Message edited: [%v]\n", ev.Msg)
					continue
				}

				// #User
				var userName string
				if ev.Msg.BotID != "" {
					userName = info.GetBotByID(ev.Msg.BotID).Name
					if userName == "" {
						fmt.Printf("Cannot fetch BotID: %v \n", ev.Msg)
						continue
					}
				} else {
					userName = info.GetUserByID(ev.User).Name
					if userName == "" {
						fmt.Printf("Cannot fetch userName: %v \n", ev.Msg)
						continue
					}
				}

				// #Channel
				channelID := ev.Msg.Channel
				ch := info.GetChannelByID(channelID)
				if ch == nil {
					// -- Private Message --
					// Do not archive private messages - only channel names
					fmt.Println("Couldnt retrieve channelname")
					fmt.Println(ev.Msg)
					continue
				} else {
					//ev.Msg.Channel = ch.Name
					msg := fmt.Sprintf("%s [%s] %s: %s", ch.Name, timeconvert(ev.Msg.Timestamp), userName, ev.Msg.Text) // DEBUG

					fmt.Printf("MessageEvent: %s \n", msg) // DEBUG
					if ev.Msg.Text == "" {
						fmt.Println("MultiLine?")
						fmt.Println(ev.Msg)
						fmt.Println(ev.Msg.Members)
					}
					archiveMsg(ev.Msg, *ch)
				}
			case *slack.PresenceChangeEvent:
			case *slack.LatencyReport:
			case *slack.RTMError:
			case *slack.InvalidAuthEvent:
			case *slack.ChannelCreatedEvent:
				fmt.Println("ChannelCreatedEvent received")
				continue
				//TODO: investigate why code below throws error
				channelName := ev.Channel.Name
				channelID := ev.Channel.ID
				fmt.Printf("New Channel created: %s - %s", channelName, channelID)
				info := rtm.GetInfo()
				//var ch *slack.Channel
				ch := info.GetChannelByID(channelID)
				if ch == nil {
					log.Printf("Error channel is nil: %s \r\n", channelName)

					x, y := rtm.GetChannelInfo(channelName)
					if y != nil {
						fmt.Println(y)
					}
					fmt.Printf("%v", x)
					break
				}
				watchChannel(*ch)
			default:
			}
		}
	}
}

func watchChannel(ch slack.Channel) {
	allChannels = append(allChannels, ch)
	log.Printf("Added New Channel #%s\n", ch.Name)
}

// Put message on MessageChannel
func archiveMsg(message slack.Msg, channel slack.Channel) {
	chMessages <- message
}

func timeconvert(value string) string {
	i, err := strconv.ParseFloat(value, 64)
	if err != nil {
		panic(err)
	}

	tm := time.Unix(int64(i), 0)
	return tm.String()
}
