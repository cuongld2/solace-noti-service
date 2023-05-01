package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/slack-go/slack"
	"solace.dev/go/messaging"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

type ChannelList struct {
	Ok       bool      `json:"ok"`
	Channels []Channel `json:"channels"`
}

type ChannelCreated struct {
	Ok      bool    `json:"ok"`
	Channel Channel `json:"channel"`
}

type Channel struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func assertEqual(a interface{}, b interface{}, message string) {
	if a == b {
		return
	}
	if len(message) == 0 {
		message = fmt.Sprintf("%v != %v", a, b)
	}
	log.Fatal("not equal when checking status code")
}

// Define Topic Prefix
const TopicPrefix = "services/meetings"

func MessageHandlerEuro(message message.InboundMessage) {
	var messageBody string

	if payload, ok := message.GetPayloadAsString(); ok {
		messageBody = payload
	} else if payload, ok := message.GetPayloadAsBytes(); ok {
		messageBody = string(payload)
	}

	var slackCategory string

	category, ok := message.GetProperty("category")

	if ok {
		slackCategory = fmt.Sprintf("%v", category)
	}
	fmt.Println("Slack category is: " + slackCategory)

	conversationListURL := "https://slack.com/api/conversations.list"

	// Create a Bearer string by appending string access token
	bearer := "Bearer " + getEnv("SLACK_USER_TOKEN", "slack_user_token_value")
	req, err := http.NewRequest("GET", conversationListURL+"?types=private_channel", nil)
	if err != nil {
		log.Println("Error setting the request http")
	}

	// add authorization header to the req
	req.Header.Add("Authorization", bearer)

	client1 := &http.Client{}
	res, err := client1.Do(req)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {

		fmt.Printf("Error parsing response body: %s\n", err)
		os.Exit(1)

	}

	var channelsList ChannelList
	json.Unmarshal(resBody, &channelsList)

	count := 0
	slackChannelId := ""

	for l := range channelsList.Channels {
		fmt.Printf("Id = %v, Name = %v", channelsList.Channels[l].Id, channelsList.Channels[l].Name)
		if slackCategory == channelsList.Channels[l].Name {

			fmt.Printf("This category channel is already existed : " + channelsList.Channels[l].Name)

			count++
			slackChannelId = channelsList.Channels[l].Id
		}
	}

	if count == 0 {
		createChannelURL := "https://slack.com/api/conversations.create?is_private=true&name=" + slackCategory

		reqCreateChannel, err := http.NewRequest("POST", createChannelURL, nil)
		if err != nil {
			log.Println("Error setting the request http")
		}
		reqCreateChannel.Header.Add("Authorization", bearer)

		client2 := &http.Client{}
		resCreateChannel, err := client2.Do(reqCreateChannel)
		if err != nil {
			log.Println("Error on response.\n[ERROR] -", err)
		}
		defer resCreateChannel.Body.Close()

		assertEqual(resCreateChannel.StatusCode, 200, "Check if response of creating new channel equals 200")

		resBodyChannelCreated, err := ioutil.ReadAll(resCreateChannel.Body)
		if err != nil {

			fmt.Printf("Error parsing response body: %s\n", err)
			os.Exit(1)

		}

		var channelCreated ChannelCreated
		json.Unmarshal(resBodyChannelCreated, &channelCreated)

		addBotToChannelURL := "https://slack.com/api/conversations.invite?channel=" + channelCreated.Channel.Id + "&users=" + getEnv("SLACK_BOT_ID", "slack_bot_id")

		reqAddBot, err := http.NewRequest("POST", addBotToChannelURL, nil)
		if err != nil {
			log.Println("Error setting the request http")
		}
		reqAddBot.Header.Add("Authorization", bearer)

		client3 := &http.Client{}
		resAddBot, err := client3.Do(reqAddBot)
		if err != nil {
			log.Println("Error on response.\n[ERROR] -", err)
		}
		defer resAddBot.Body.Close()

		assertEqual(resAddBot.StatusCode, 200, "Check http status code")

		// testText := "New meeting has been created by user is: donald with category is: standup_meeting and title is: standup_meeting_01_05_2023"

		api := slack.New(getEnv("SLACK_BOT_TOKEN", "token"))

		api.PostMessage(channelCreated.Channel.Id, slack.MsgOptionText(messageBody, false))

	}

	if count == 1 {

		fmt.Println("bbbbbbbbbbbbb")

		api := slack.New(getEnv("SLACK_BOT_TOKEN", "token"))

		api.PostMessage(slackChannelId, slack.MsgOptionText(messageBody, false))

	}
}

func getEnv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func main() {

	// Configuration parameters
	brokerConfig := config.ServicePropertyMap{
		config.TransportLayerPropertyHost:                getEnv("TransportLayerPropertyHost", "tcps://"),
		config.ServicePropertyVPNName:                    getEnv("ServicePropertyVPNName", "brokerName"),
		config.AuthenticationPropertySchemeBasicUserName: getEnv("AuthenticationPropertySchemeBasicUserName", "solace-cloud-client"),
		config.AuthenticationPropertySchemeBasicPassword: getEnv("AuthenticationPropertySchemeBasicPassword", "password"),
	}
	messagingService, err := messaging.NewMessagingServiceBuilder().FromConfigurationProvider(brokerConfig).WithTransportSecurityStrategy(config.NewTransportSecurityStrategy().WithoutCertificateValidation()).
		Build()

	if err != nil {
		panic(err)
	}

	// Connect to the messaging serice
	if err := messagingService.Connect(); err != nil {
		panic(err)
	}

	fmt.Println("Connected to the broker? ", messagingService.IsConnected())

	//  Build a Direct Message Receiver
	directReceiver, err := messagingService.CreateDirectMessageReceiverBuilder().
		WithSubscriptions(resource.TopicSubscriptionOf(TopicPrefix + "/senderUserId/*/meetingCategory/*/meetingTitle/>")).
		Build()

	if err != nil {
		panic(err)
	}

	// Start Direct Message Receiver
	if err := directReceiver.Start(); err != nil {
		panic(err)
	}

	fmt.Println("Direct Receiver running? ", directReceiver.IsRunning())

	for 1 != 0 {

		if regErr := directReceiver.ReceiveAsync(MessageHandlerEuro); regErr != nil {
			panic(regErr)
		}

	}

}
