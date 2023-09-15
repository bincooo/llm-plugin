package chain

import (
	"fmt"
	autotypes "github.com/bincooo/AutoAI/types"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/bincooo/llm-plugin/internal/types"
	"strings"
)

const MaxOnlineCount = 30

type OnlineInterceptor struct {
	autotypes.BaseInterceptor
}

func (c *OnlineInterceptor) Before(bot autotypes.Bot, ctx *autotypes.ConversationContext) (bool, error) {
	cacheOnline(ctx)
	if strings.Contains(ctx.Prompt, "[online]") {
		online := make([]string, 0)
		for _, o := range store.GetOnline(ctx.Id) {
			online = append(online, fmt.Sprintf(`{"qq":"%s", "name": "%s"}`, o["id"], o["name"]))
		}
		ctx.Prompt = strings.Replace(ctx.Prompt, "[online]", "["+strings.Join(online, ",")+"]", -1)
	}

	return true, nil
}

func cacheOnline(ctx *autotypes.ConversationContext) {
	online := store.GetOnline(ctx.Id)
	args := ctx.Data.(types.ConversationContextArgs)
	// 如果已在线列表中，先删除后加入到结尾
	for i, ol := range online {
		if ol["id"] != args.Current {
			continue
		}

		if len(online) == 1 {
			online = make([]map[string]string, 0)
		} else {
			online = append(online[:i], online[i+1:]...)
		}

		break
	}

	// 加入在线列表
	online = append(online, map[string]string{
		"id":   args.Current,
		"name": args.Nickname,
	})

	// 控制最大在线人数
	if len(online) > MaxOnlineCount {
		online = online[len(online)-MaxOnlineCount:]
	}
	store.CacheOnline(ctx.Id, online)
}
