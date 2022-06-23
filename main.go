package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	SLACK_BOT_TOKEN := os.Getenv("SLACK_BOT_TOKEN")
	SLACK_APP_TOKEN := os.Getenv("SLACK_APP_TOKEN")
	//SLACK_SIGNING_SECRET := os.Getenv("SLACK_SIGNING_SECRET")
	//LF_API_KEY := os.Getenv("LF_API_KEY")
	//LF_API_SECRET := os.Getenv("LF_API_SECRET")
	//LF_USERNAME := os.Getenv("LF_USERNAME")
	//LF_PASSWORD_HASH := os.Getenv("LF_PASSWORD_HASH")
	slack_api := slack.New(
		SLACK_BOT_TOKEN,
		slack.OptionAppLevelToken(SLACK_APP_TOKEN),
	)

	client := socketmode.New(
		slack_api,
	)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)
					continue
				}
				fmt.Printf("Event received: %+v\n", eventsAPIEvent)

				client.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						_, _, err := slack_api.PostMessage(ev.Channel, slack.MsgOptionText("Yes", false))
						if err != nil {
							fmt.Printf("failed posting message: %v\n", evt)
						}
					}
				default:
					client.Debugf("unsupported Events API event received")
				}

			default:
				fmt.Fprintf(os.Stderr, "unexpected event type received: %s\n", evt.Type)
			}
		}
	}()

	//users, err := slack_api.GetUsers()
	//if err != nil {
	//	fmt.Printf("%s\n", err)
	//	return
	//}
	//for _, user := range users {
	//	fmt.Printf("ID: %s, Name: %s\n", user.ID, user.Name)
	//}

	//network := lastfm.New(LF_API_KEY, LF_API_SECRET)

	//result, _ := network.Artist.GetTopTracks(lastfm.P{"artist": "Future"})
	//for _, track := range result.Tracks {
	//	fmt.Println(track.Name)
	//}
	client.Run()
}
