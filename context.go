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

	global := repo.GetGlobal()
	if bot == "" {
		bot = global.Bot
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

	if strings.HasPrefix(bot, vars.Claude) {
		if bot == vars.Claude+"-web" {
			model = claudevars.Model4WebClaude2
		} else {
			model = claudevars.Model4Slack
		}
		bot = vars.Claude
	}

	maxTokens := 4000
	if tokens[0].MaxTokens != 0 {
		maxTokens = tokens[0].MaxTokens
	}

	args := types.ConversationContextArgs{
		Pid: "-1",
	}
	cctx := autotypes.ConversationContext{
		Id:        key,
		Bot:       bot,
		MaxTokens: maxTokens,
		Chain:     BaseChain,
		Model:     model,
		Proxy:     global.Proxy,
	}

	if bot == vars.OpenAIAPI {
		if tokens[0].AppId != "" {
			cctx.Model = tokens[0].AppId
		}
	}

	if bot == vars.OpenAIWeb {
		// 检查失效
		cctx.BaseURL = "https://ai.fakeopen.com/api"
	}

	if bot == vars.Claude {
		cctx.AppId = tokens[0].AppId
	}

	// 默认预设
	if global.Role != "" {
		suf := ""
		if bot == vars.Claude && model == claudevars.Model4WebClaude2 {
			suf = "-web"
		}
		preset := repo.GetRole("", global.Role, bot+suf)
		if preset == nil {
			logrus.Warn("预设`", global.Role, "`不存在")
		} else if preset.Type != bot+suf {
			logrus.Warn("预设`", global.Role, "`类型不匹配, 需要（", bot, "）实际为（", preset.Type, "）")
		} else {
			args.Pid = preset.Id
			cctx.Preset = preset.Content
			cctx.Format = preset.Message
			if preset.Chain != "" {
				cctx.Chain += preset.Chain
			}
		}
	}

	if tokens[0].BaseURL != "" {
		cctx.BaseURL = tokens[0].BaseURL
		logrus.Infoln("[MiaoX] - AI转发地址： ", cctx.BaseURL)
	}

	cctx.Token = tokens[0].Token
	cctx.Data = args

	updateConversationContext(cctx)
	logrus.Infoln("[MiaoX] - 创建新的ConversationContext： ", key)
	return cctx, nil
}

func parseMessage(ctx *zero.Ctx) string {
	// and more...
	text := ctx.ExtractPlainText()
	picture := tryPicture(ctx)
	if picture != "" {
		imgdata, err := web.GetData(picture)
		if err != nil {
			logrus.Error(err)
			goto label
		}
		path := "data/" + uuid.NewString() + ".jpg"
		err = os.WriteFile(path, imgdata, 0666)
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

// tryPicture 消息含有图片返回
func tryPicture(ctx *zero.Ctx) string {
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
