package llm

import (
	"errors"
	"github.com/FloatTech/floatbox/web"
	"github.com/google/uuid"
	"github.com/wdvxdr1123/ZeroBot/message"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bincooo/chatgpt-adapter/vars"
	"github.com/bincooo/llm-plugin/internal/repo"
	"github.com/bincooo/llm-plugin/internal/types"
	"github.com/sirupsen/logrus"

	autotypes "github.com/bincooo/chatgpt-adapter/types"
	claudevars "github.com/bincooo/claude-api/vars"
	zero "github.com/wdvxdr1123/ZeroBot"
)

var (
	mu           sync.Mutex
	contextStore = make(map[string]autotypes.ConversationContext)
)

const (
	BaseChain = ""
)

func deleteConversationContext(ctx *zero.Ctx) {
	mu.Lock()
	defer mu.Unlock()

	var id int64 = 0
	if ctx.Event.GroupID == 0 {
		id = ctx.Event.UserID
	} else {
		id = ctx.Event.GroupID
	}
	key := strconv.FormatInt(id, 10)
	delete(contextStore, key)
}

func getId(ctx *zero.Ctx) string {
	var id int64 = 0
	if ctx.Event.GroupID == 0 {
		id = ctx.Event.UserID
	} else {
		id = ctx.Event.GroupID
	}
	return strconv.FormatInt(id, 10)
}

func updateConversationContext(cctx autotypes.ConversationContext) {
	mu.Lock()
	defer mu.Unlock()
	contextStore[cctx.Id] = cctx
	logrus.Infoln("[MiaoX] - 更新ConversationContext： ", cctx.Id)
}

func createConversationContext(ctx *zero.Ctx, bot string) (autotypes.ConversationContext, error) {
	key := getId(ctx)

	if cctx, ok := contextStore[key]; ok {
		logrus.Infoln("[MiaoX] - 获取缓存ConversationContext： ", key)
		return cctx, nil
	}

	g := repo.GetGlobal()
	if bot == "" {
		bot = g.Bot
	}

	model := ""
	if strings.HasPrefix(bot, vars.Bing) {
		expr := bot[len(bot)-1:]
		switch expr {
		case "b":
			model = "Balanced"
		case "p":
			model = "Precise"
		case "s":
			model = "Sydney"
		default:
			model = "Creative"
		}
		bot = vars.Bing
	}

	tokens, err := repo.FindTokens("", bot)
	if err != nil {
		return autotypes.ConversationContext{}, errors.New("查询凭证失败, 请先添加`" + bot + "`凭证")
	}
	if len(tokens) == 0 {
		return autotypes.ConversationContext{}, errors.New("无可用的凭证")
	}
	dbToken := tokens[0]

	if strings.HasPrefix(bot, vars.Claude) {
		if bot == vars.Claude+"-web" {
			model = claudevars.Model4WebClaude2
		} else {
			model = claudevars.Model4Slack
		}
		bot = vars.Claude
	}

	maxTokens := 4000
	if dbToken.MaxTokens != 0 {
		maxTokens = dbToken.MaxTokens
	}

	args := types.ConversationContextArgs{
		Rid: "-1",
		Tid: "-1",
	}

	logrus.Infoln("[MiaoX] - 代理地址： ", g.Proxy)
	cctx := autotypes.ConversationContext{
		Id:          key,
		Bot:         bot,
		MaxTokens:   maxTokens,
		Chain:       BaseChain,
		Model:       model,
		Proxy:       g.Proxy,
		Temperature: .8,
	}

	if bot == vars.OpenAIAPI {
		if dbToken.AppId != "" {
			cctx.Model = dbToken.AppId
		}
	}

	if bot == vars.OpenAIWeb {
		cctx.BaseURL = "https://ai.fakeopen.com/api"
	}

	if bot == vars.Claude {
		cctx.AppId = dbToken.AppId
	}

	// 默认预设
	if g.Role != "" {
		suf := ""
		if bot == vars.Claude && model == claudevars.Model4WebClaude2 {
			suf = "-web"
		}
		role := repo.GetRole("", g.Role, bot+suf)
		if role == nil {
			logrus.Warn("预设`", g.Role, "`不存在")
		} else if role.Type != bot+suf {
			logrus.Warn("预设`", g.Role, "`类型不匹配, 需要（", bot, "）实际为（", role.Type, "）")
		} else {
			args.Rid = role.Id
			cctx.Preset = role.Content
			cctx.Format = role.Message
			if role.Chain != "" {
				cctx.Chain += role.Chain
			}
		}
	}

	args.Tid = dbToken.Id
	cctx.Data = args
	cctx.Token = dbToken.Token
	if dbToken.BaseURL != "" {
		cctx.BaseURL = dbToken.BaseURL
		logrus.Infoln("[MiaoX] - AI转发地址： ", cctx.BaseURL)
	}

	updateConversationContext(cctx)
	logrus.Infoln("[MiaoX] - 创建新的ConversationContext： ", key)
	return cctx, nil
}

func parseMessage(ctx *zero.Ctx, images bool) string {
	// and more...
	text := ctx.ExtractPlainText()
	if !images || strings.TrimSpace(text) == "" {
		return text
	}

	if image := tryImage(ctx); image != "" {
		data, err := web.GetData(image)
		if err != nil {
			logrus.Error(err)
			goto label
		}
		path := "data/" + uuid.NewString() + ".jpg"
		err = os.WriteFile(path, data, 0666)
		if err != nil {
			logrus.Error(err)
			goto label
		}
		text = "{image:" + path + "}\n" + text
		go func() {
			time.Sleep(30 * time.Second)
			_ = os.Remove(path)
		}()
	}
label:
	return text
}

// tryImage 消息含有图片返回
func tryImage(ctx *zero.Ctx) string {
	messages := ctx.Event.Message
	for _, msg := range messages {
		if msg.Type != "reply" {
			continue
		}
		replyMessage := ctx.GetMessage(message.NewMessageIDFromString(msg.Data["id"]))
		for _, e := range replyMessage.Elements {
			if e.Type != "image" {
				continue
			}
			if u := e.Data["url"]; u != "" {
				return u
			}
		}
		break
	}
	return ""
}
