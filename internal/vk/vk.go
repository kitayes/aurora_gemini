package vk

import (
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/events"
	longpoll "github.com/SevereCloud/vksdk/v2/longpoll-bot"
)

func NewVK(token string) *api.VK {
	return api.NewVK(token)
}

func NewLongPoll(vk *api.VK, groupID int) (*longpoll.LongPoll, error) {
	return longpoll.NewLongPoll(vk, groupID)
}

type Message = events.MessageNewObject
