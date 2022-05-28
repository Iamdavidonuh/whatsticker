package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/deven96/whatsticker/handler"
	_ "github.com/mattn/go-sqlite3"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var client *whatsmeow.Client
var replyTo *bool

var commands = map[string]struct{}{
	"stickerize deven96": {},
	"stickerize":         {},
}

func loginNewClient() {
	// No ID stored, new login
	qrChan, _ := client.GetQRChannel(context.Background())
	err := client.Connect()
	if err != nil {
		panic(err)
	}
	for evt := range qrChan {
		if evt.Event == "code" {
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			fmt.Println("QR code:", evt.Code)
		} else {
			fmt.Println("Login event:", evt.Event)
		}
	}
}

func captionIsCommand(caption string) bool {
	_, ok := commands[strings.TrimSpace(strings.ToLower(caption))]
	return ok
}

func listenForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func eventHandler(evt interface{}) {
	switch eventInfo := evt.(type) {
	case *events.ConnectFailure, *events.ClientOutdated:
		fmt.Println("Killing due to client related issues")
		os.Exit(1)
	case *events.StreamReplaced:
		fmt.Println("Started another stream with the same device session")
		//    os.Exit(1)
	case *events.OfflineSyncCompleted:
		fmt.Println("Offline sync completed")
	case *events.Message:
		extended := eventInfo.Message.GetExtendedTextMessage()
		quotedMsg := extended.GetContextInfo().GetQuotedMessage()
		quotedImage := quotedMsg.GetImageMessage()
		quotedVideo := quotedMsg.GetVideoMessage()
		quotedText := extended.GetText()

		imageMatch := captionIsCommand(eventInfo.Message.GetImageMessage().GetCaption())
		videoMatch := captionIsCommand(eventInfo.Message.GetVideoMessage().GetCaption())
		// check if quoted message with correct caption references media
		quotedMatch := captionIsCommand(quotedText) &&
			(quotedImage != nil || quotedVideo != nil)

		if imageMatch || videoMatch || quotedMatch {
			if quotedMatch {
				// replace the actual message struct with quoted media
				if quotedImage != nil {
					eventInfo.Info.MediaType = "image"
					eventInfo.Message.ImageMessage = quotedImage
				} else if quotedVideo != nil {
					// FIXME: gif quoted message just gets set as video
					// currently does not matter as much since both
					// use the same Video handler
					eventInfo.Info.MediaType = "video"
					eventInfo.Message.VideoMessage = quotedVideo
				}
			}
			go handler.Run(client, eventInfo, *replyTo)
		}
	}
}

func main() {
	loglevel := flag.String("log-level", "INFO", "Set log level to one of (INFO/DEBUG)")
	replyTo = flag.Bool("reply-to", false, "Set to true to highlight messages to respond to")
	flag.Parse()
	dbLog := waLog.Stdout("Database", *loglevel, true)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err := sqlstore.New("sqlite3", "file:db/examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", *loglevel, true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)
	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true

	if client.Store.ID == nil {
		loginNewClient()
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	listenForCtrlC()
	client.Disconnect()
}
