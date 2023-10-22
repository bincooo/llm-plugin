package llm

import (
	"context"
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
	wapi "github.com/bincooo/openai-wapi"
	zero "github.com/wdvxdr1123/ZeroBot"
)

var (
	mu           sync.Mutex
	contextStore = make(map[string]autotypes.ConversationContext)
)

const (
	Poe       = "poe"
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

	// POE
	if strings.HasPrefix(bot, Poe) {
		switch bot {
		case Poe + "-gpt3.5":
			model = "gpt-3.5-turbo"
		case Poe + "-gpt4":
			model = "gpt-4"
		case Poe + "-gpt4-32k":
			model = "gpt-4-32k"
		case Poe + "-claude+":
			model = "Claude+"
		case Poe + "-claude100k":
			model = "Claude-instant-100k"
		default:
			model = "gpt-3.5-turbo"
		}
		bot = Poe
	}

	tokens, err := repo.FindTokens(bot)
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

	args := types.ConversationContextArgs{
		PresetId: "-1",
		TokenId:  "-1",
	}
	cctx := autotypes.ConversationContext{
		Id:        key,
		Bot:       bot,
		MaxTokens: global.MaxTokens,
		Chain:     BaseChain,
		Model:     model,
		Proxy:     global.Proxy,
	}

	if bot == vars.OpenAIAPI {
		//	// 检查余额
		//	if e := checkApiOpenai(*tokens[0], global.Proxy); e != nil {
		//		return cctx, e
		//	}
		if tokens[0].AppId != "" {
			cctx.Model = tokens[0].AppId
		}
	}

	if bot == vars.OpenAIWeb {
		// 检查失效
		cctx.BaseURL = "https://ai.fakeopen.com/api"
		if err := checkWebOpenai(tokens[0], global.Proxy); err != nil {
			return cctx, err
		}
		//// 为空，尝试登陆
		//if tokens[0].Token == "" {
		//	if err := loginWebOpenai(*tokens[0], global); err != nil {
		//		return cctx, err
		//	}
		//}
	}

	if bot == vars.Claude {
		cctx.AppId = tokens[0].AppId
	}

	if bot == vars.Bing {
		cctx.BaseURL = global.NbServ
	}

	// 默认预设
	if global.Preset != "" {
		suf := ""
		if bot == vars.Claude && model == claudevars.Model4WebClaude2 {
			suf = "-web"
		}
		preset := repo.GetPresetScene("", global.Preset, bot+suf)
		if preset == nil {
			logrus.Warn("预设`", global.Preset, "`不存在")
		} else if preset.Type != bot+suf {
			logrus.Warn("预设`", global.Preset, "`类型不匹配, 需要（", bot, "）实际为（", preset.Type, "）")
		} else {
			args.PresetId = preset.Id
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

	args.TokenId = tokens[0].Id
	cctx.Token = tokens[0].Token
	cctx.Data = args

	updateConversationContext(cctx)
	logrus.Infoln("[MiaoX] - 创建新的ConversationContext： ", key)
	return cctx, nil
}

// 登陆网页版
func loginWebOpenai(token repo.Token, global repo.Global) error {
	t, err := wapi.WebLogin(token.Email, token.Passwd, global.Proxy)
	if err != nil {
		return errors.New("OpenAI WEB `" + token.Key + "`登陆失败: " + err.Error())
	}
	token.Token = t
	token.Expire = time.Now().Add(15 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	repo.UpdateToken(token)
	return nil
}

// 检查余额
func checkApiOpenai(token repo.Token, proxy string) error {
	if billing, _ := wapi.Query(context.Background(), token.Token, proxy); billing == nil || billing.System-billing.Soft < 0 {
		return errors.New("Err: `" + token.Key + "`凭证余额为0")
	}
	return nil
}

// 检查过期时间
func checkWebOpenai(token *repo.Token, proxy string) error {
	if token.Expire != "" && token.Expire != "-1" {
		expire, err := time.Parse("2006-01-02 15:04:05", token.Expire)
		if err != nil {
			return errors.New("warning：[" + token.Key + "] `" + token.Expire + "`过期日期解析有误")
		}

		if expire.Before(time.Now()) {
			// 已过期
			t, err := wapi.WebLogin(token.Email, token.Passwd, proxy)
			if err != nil {
				return errors.New("OpenAI WEB `" + t + "`登陆失败: " + err.Error())
			}
			token.Token = t
			token.Expire = time.Now().Add(14 * 24 * time.Hour).Format("2006-01-02 15:04:05")
			repo.UpdateToken(*token)
		}
	}
	return nil
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
