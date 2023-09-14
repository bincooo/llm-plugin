package chain

import (
	"github.com/bincooo/AutoAI/types"
	pTypes "github.com/bincooo/llm-plugin/types"
	"strings"
)

type ArgsInterceptor struct {
	types.BaseInterceptor
}

func (c *ArgsInterceptor) Before(bot types.Bot, ctx *types.ConversationContext) bool {
	args := ctx.Data.(pTypes.ConversationContextArgs)
	if strings.Contains(ctx.Prompt, "[qq]") {
		ctx.Prompt = strings.Replace(ctx.Prompt, "[qq]", args.Current, -1)
	}
	if strings.Contains(ctx.Prompt, "[name]") {
		ctx.Prompt = strings.Replace(ctx.Prompt, "[name]", args.Nickname, -1)
	}
	return true
}
