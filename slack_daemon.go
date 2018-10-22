package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nlopes/slack"
	logrus "github.com/sirupsen/logrus"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var api *slack.Client
var msgTimestamps []string
var channels []slack.Channel
var evtChannelInfo *slack.ChannelInfoEvent
var chMessages chan message
var slackToken string
var allChannels []slack.Channel
var rtm *slack.RTM
var outputFolder string
var outputType string
var db *sql.DB

/*
// TODO
- A bot can post multiple messages at a time
- if strings.Contains(message.Text, "@") {
- [14:45]  jarko added an integration to this channel: incoming-webhook --> Crash
*/

func main() {
	parseFlags()

	//chTopics := make(chan slack.Channel)
	chMessages = make(chan message, 100)

	api = slack.New(slackToken)
	// If you set debugging, it will log all requests to the console
	// Useful when encountering issues
	//api.OptionDebug(false)
	slack.OptionDebug(false)

	rtm = api.NewRTM()
	go rtm.ManageConnection()

	logger := log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)
	slack.OptionLog(logger)

	// Read & print all messages
	go fetchEvents_crashHandler(readMessages)

	// Fetch all events
	fetchEvents_crashHandler(fetchEvents)
}

func fetchEvents_crashHandler(f func()) {
	defer func() {
		log.Println("fetchEvents_Crashandler intervention!") // recover
		log.Println(recover())                               // recover
		fetchEvents_crashHandler(f)
	}()
	f()
}

func parseFlags() {
	var tokenFlag = flag.String("token", "", "your slack token")
	var logLevelFlag = flag.String("logLevel", "info", "logrus log level")
	var outputFolderFlag = flag.String("outputFolder", "log", "the output folder location - default ./log")
	var outputTypeFlag = flag.String("outputType", "txt", "output type: txt | sqlite - default \"txt\"")

	flag.Parse()

	if *tokenFlag == "" {
		log.Fatal("Parameter token not given.")
	}
	slackToken = *tokenFlag

	if *outputFolderFlag == "" {
		log.Println("Writing to ./log")
	} else {
		outputFolder = *outputFolderFlag
		log.Printf("Writing to %s", outputFolder)
	}

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

	if *outputTypeFlag == "txt" {
		outputType = "txt"
	} else {
		outputType = "influxDb"
	}
	log.Printf("OutputType = %s", outputType)
}

func readMessages() {
	for {
		if chMessages == nil {
			continue
		}
		message := <-chMessages
		if strings.Contains(message.text, "<@") {
			// cast username
		}
		output := fmt.Sprintf("[%s] @%s: %s \r\n", message.timestamp, message.username, message.text)
		storeMessage(message.channel, output)
	}
}

func storeMessage(channel string, message string) {
	log.Println("Storing Message")

	switch outputType {
	case "txt":
		write2file(channel, message)
		break
	case "influxDb":
		write2dB(channel, message)
		break
	default:
		log.Println("Unkown output-type - stopping")
		os.Exit(0)
		break
	}
}

func InitDB(filepath string) *sql.DB {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		log.Println("InitDb - Error")
		panic(err)
	}
	if db == nil {
		log.Println("InitDb - db is nil")
		panic("db nil")
	}
	return db
}

func CreateTable(db *sql.DB, tableName string) {
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY, message TEXT)", tableName)
	log.Printf("Statement: %s", sql)
	_, err := db.Exec(sql)
	if err != nil {
		log.Println("CreateTable - Error")
		log.Println(err)
		//panic(err)
	} else {
		log.Println("CreateTable")
	}
}

func write2dB(channel string, message string) {
	log.Println("Write2DB")
	filename := fmt.Sprintf("%s.db3", "slack_logs")
	path := filepath.Join(outputFolder, filename)

	var err error //dirty fix

	db := InitDB(path)
	defer db.Close()

	CreateTable(db, channel)

	//StoreItem(db, items)

	// insert
	sql := fmt.Sprintf("INSERT INTO %s(message) values(?,?)", channel)
	stmt, err := db.Prepare(sql)
	log.Printf("Statement: %s", stmt)
	if err != nil {
		//panic(err)
		log.Println(err)
	} else {
		log.Println("Insert")
	}

	_, err = stmt.Exec(message)
	if err != nil {
		panic(err)
	}
}

func write2file(channel string, message string) {
	filename := fmt.Sprintf("%s.log", channel)
	path := filepath.Join(outputFolder, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
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
	// // http://codesamplez.com/programming/golang-error-handling
	// defer func() {
	// 	if err := recover(); err != nil {
	// 		fmt.Println("Error: ", err)
	// 		var msg message
	// 		msg.username = "ErrorMan"
	// 		msg.text = fmt.Sprintf("%v", err)
	// 		msg.channel = "error"
	// 		archiveMsg(msg)
	// 		//fetchEvents() // Crash occurs here
	// 	}
	// }()

	excludeArchivedChannels := true
	allChannels, _ = rtm.GetChannels(excludeArchivedChannels)

	log.Println("Watching:")
	for _, channel := range allChannels {
		log.Printf("Channel #%s\n", channel.Name)
	}

	for {
		select {
		case slackMsg := <-rtm.IncomingEvents:
			switch ev := slackMsg.Data.(type) {
			case *slack.ConnectedEvent:
			case *slack.MessageEvent:
				info := rtm.GetInfo()
				var msg message

				// Debug
				if ev.Msg.Text != "" {
					var err message
					err.text = fmt.Sprintf("%v", ev.Msg)
					err.channel = "debug"
					archiveMsg(err)
				}

				// #Message
				switch ev.Msg.SubType {
				case "":
					fmt.Printf("New Message: [%v]\n[%v]", ev.Msg, ev)
				case "message_changed":
					fmt.Printf("Message edited: [%v]\n[%v]", ev.Msg, ev)
					fmt.Printf("Editted @ %s", timeconvert(ev.Msg.EventTimestamp))
					continue
				case "message_deleted":
					fmt.Printf("Message deleted: [%v]\n[%v]", ev.Msg, ev)
					fmt.Printf("Deleted @ %s", timeconvert(ev.Msg.DeletedTimestamp))
					continue
				}

				msg.timestamp = timeconvert(ev.Timestamp)

				// #User
				userName := GetUsername(ev)
				fmt.Printf("[%s] ", userName)
				msg.username = userName

				// #Channel
				var ch *slack.Channel
				if ev.Msg.Channel != "" {
					channelID := ev.Msg.Channel
					ch = info.GetChannelByID(channelID)
					if ch == nil {
						// No channel - private post
						msg.channel = "private"
					} else {
						switch ch.IsChannel {
						case true:
							fmt.Printf("[Channel:#%s]\n", ch.Name)
							msg.channel = ch.Name
						case false:
							// unreachable code > channel always has name
							fmt.Println("not posting in a channel!")
						}
					}
				} else {
					// No channel - private post
					msg.channel = "noChannel"
				}
				fmt.Printf("\n")

				// #Text/Message
				if ev.Msg.Text != "" {
					msg.text = ev.Msg.Text
				} else {
					msg.text = "Couldnt fetch text"
					if len(ev.Msg.Attachments) != 0 {
						msg.text = fmt.Sprintf("%v", ev.Msg.Attachments)
						fmt.Sprintf("%v \n", ev.Msg.Attachments)
					}
				}

				// Archive!
				archiveMsg(msg)
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

func GetUsername(ev *slack.MessageEvent) string {
	info := rtm.GetInfo()
	var userName string
	if ev.Msg.BotID != "" {
		userName = info.GetBotByID(ev.Msg.BotID).Name
		if userName == "" {
			fmt.Printf("Cannot fetch BotID: %v \n", ev.Msg)
		}
	} else if ev.User != "" {
		userName = info.GetUserByID(ev.User).Name
		if userName == "" {
			fmt.Printf("Cannot fetch userName: %v \n", ev.Msg)
		}
	} else {
		fmt.Printf("Cannot fetch userName: %v \n", ev.Msg)
	}
	return userName
}

func watchChannel(ch slack.Channel) {
	allChannels = append(allChannels, ch)
	log.Printf("Added New Channel #%s\n", ch.Name)
}

// Put message on MessageChannel
func archiveMsg(msg message) {
	chMessages <- msg
}

func timeconvert(value string) string {
	if value == "" {
		return ""
	}
	i, err := strconv.ParseFloat(value, 64)
	if err != nil {
		panic(err)
	}

	tm := time.Unix(int64(i), 0)
	return tm.String()
}
