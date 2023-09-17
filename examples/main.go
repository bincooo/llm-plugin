package main

import (
	"github.com/bincooo/AutoAI"
	"github.com/bincooo/AutoAI/types"
	"github.com/bincooo/AutoAI/vars"
)

func main() {
	manager := AutoAI.NewBotManager()
	context := Context()
	context.Prompt = "嗨"
	manager.Reply(context, func(response types.PartialResponse) {

	})
}

func Context() types.ConversationContext {
	return types.ConversationContext{
		Id:  "1008611",
		Bot: vars.Bing,
		//Bot:     vars.OpenAIWeb,
		Token:   "",
		Preset:  "",
		Format:  "",
		Chain:   "replace,cache,assist",
		BaseURL: "https://ai.fakeopen.com/api",
		//Model:   edge.Sydney,
	}
}
