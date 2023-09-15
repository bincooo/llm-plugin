package chain

import (
	autotypes "github.com/bincooo/AutoAI/types"
	"github.com/bincooo/llm-plugin/internal/types"
	"strings"
)

type ArgsInterceptor struct {
	autotypes.BaseInterceptor
}

func (c *ArgsInterceptor) Before(bot autotypes.Bot, ctx *autotypes.ConversationContext) (bool, error) {
	args := ctx.Data.(types.ConversationContextArgs)
	if strings.Contains(ctx.Prompt, "[qq]") {
		ctx.Prompt = strings.Replace(ctx.Prompt, "[qq]", args.Current, -1)
	}
	if strings.Contains(ctx.Prompt, "[name]") {
		ctx.Prompt = strings.Replace(ctx.Prompt, "[name]", args.Nickname, -1)
	}
	return true, nil
}
