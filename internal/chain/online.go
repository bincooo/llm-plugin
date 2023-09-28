package chain

import (
	autotypes "github.com/bincooo/AutoAI/types"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/bincooo/llm-plugin/internal/types"
)

const MaxOnlineCount = 30

type OnlineInterceptor struct {
	autotypes.BaseInterceptor
}

func (*OnlineInterceptor) Before(bot autotypes.Bot, ctx *autotypes.ConversationContext) (bool, error) {
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
	return true, nil
}
