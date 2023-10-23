package llm

import "github.com/bincooo/chatgpt-adapter/vars"

func validateType(bot string) bool {
	switch bot {
	case vars.OpenAIAPI,
		vars.OpenAIWeb,
		vars.Claude,
		vars.Claude + "-web",
		vars.Bing:
		return true
	default:
		return false
	}
}

func validateBot(bot string) bool {
	switch bot {
	case vars.OpenAIAPI,
		vars.OpenAIWeb,
		vars.Claude,
		vars.Claude + "-web",
		vars.Bing + "-c",
		vars.Bing + "-b",
		vars.Bing + "-p",
		vars.Bing + "-s":
		return true
	default:
		return false
	}
}
