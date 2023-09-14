package llm_plugin

import (
	"context"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/bincooo/AutoAI"
	store2 "github.com/bincooo/AutoAI/store"
	"github.com/bincooo/AutoAI/types"
	xvars "github.com/bincooo/AutoAI/vars"
	clVars "github.com/bincooo/claude-api/vars"
	"github.com/bincooo/llm-plugin/chain"
	"github.com/bincooo/llm-plugin/cmd"
	"github.com/bincooo/llm-plugin/repo"
	"github.com/bincooo/llm-plugin/repo/store"
	pTypes "github.com/bincooo/llm-plugin/types"
	"github.com/bincooo/llm-plugin/utils"
	"github.com/bincooo/llm-plugin/vars"
	wapi "github.com/bincooo/openai-wapi"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var help = `
- @Bot + 文本内容
- 昵称前缀 + 文本内容
- 预设列表
- [开启|切换]预设 + [预设名]
- 删除凭证 + [key]
- 添加凭证 + [key]:[value]
- 切换AI + [AI类型：openai-api、openai-web、claude、claude-web、
	bing-(c|b|p|s)、poe-(gpt3.5|gpt4|gpt4-32k|claude+|claude100k)]
`
var (
	engine = control.Register("miaox", &ctrl.Options[*zero.Ctx]{
		Help:              help,
		Brief:             "喵小爱 - AI适配器",
		DisableOnDefault:  false,
		PrivateDataFolder: "miaox",
	})

	//mgr types.BotManager
	lmt types.Limiter
)

func init() {
	vars.E = engine

	var err error
	if vars.Loading, err = os.ReadFile(engine.DataFolder() + "load.gif"); err != nil {
		panic(err)
	}

	lmt = AutoAI.NewCommonLimiter()
	if e := lmt.RegChain("args", &chain.ArgsInterceptor{}); e != nil {
		panic(e)
	}
	if e := lmt.RegChain("online", &chain.OnlineInterceptor{}); e != nil {
		panic(e)
	}

	engine.OnRegex(`^添加凭证\s+(\S+)`, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(insertTokenCommand)
	engine.OnRegex(`^删除凭证\s+(\S+)`, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(deleteTokenCommand)
	engine.OnFullMatch("凭证列表", zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(tokensCommand)
	engine.OnRegex(`[开启|切换]预设\s(\S+)`, zero.OnlyToMe, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(enablePresetSceneCommand)
	engine.OnRegex(`切换AI\s(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(switchAICommand)
	engine.OnFullMatch("预设列表", zero.OnlyToMe, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(presetScenesCommand)
	engine.OnPrefix("作画", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(drawCommand)
	engine.OnFullMatch("历史对话", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(historyCommand)
	engine.OnMessage(zero.OnlyToMe, repo.OnceOnSuccess, excludeOnMessage).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(conversationCommand)

	cmd.Register("/api/global", repo.GlobalService{}, cmd.NewMenu("global", "全局配置"))
	cmd.Register("/api/preset", repo.PresetService{}, cmd.NewMenu("preset", "预设配置"))
	cmd.Register("/api/token", repo.TokenService{}, cmd.NewMenu("token", "凭证配置"))

	//Run(":8082")
}

// 自定义优先级
func excludeOnMessage(ctx *zero.Ctx) bool {
	msg := ctx.MessageString()
	exclude := []string{"添加凭证 ", "删除凭证 ", "凭证列表", "开启预设 ", "切换预设 ", "预设列表", "历史对话", "切换AI ", "作画", "/", "!"}
	for _, value := range exclude {
		if strings.HasPrefix(msg, value) {
			return false
		}
	}
	return true
}

func historyCommand(ctx *zero.Ctx) {
	key := getId(ctx)
	messages := store2.GetMessages(key)
	logrus.Info(messages)
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已在后台打印"))
}

// 聊天
func conversationCommand(ctx *zero.Ctx) {
	name := ctx.Event.Sender.NickName
	if strings.Contains(name, "Q群管家") {
		return
	}

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("发生异常: "+err.Error()))
		return
	}

	cctx.Prompt = parseMessage(ctx)
	args := cctx.Data.(pTypes.ConversationContextArgs)
	args.Current = strconv.FormatInt(ctx.Event.Sender.ID, 10)
	args.Nickname = ctx.Event.Sender.NickName
	cctx.Data = args
	// 使用了poe-openai-proxy
	if cctx.Bot == Poe {
		cctx.Bot = xvars.OpenAIAPI
	}

	delay := utils.NewDelay(ctx)
	lmtHandle := func(response types.PartialResponse) {
		if response.Status == xvars.Begin {
			delay.Defer()
		}

		if len(response.Message) > 0 {
			segment := utils.StringToMessageSegment(response.Message)
			ctx.SendChain(append(segment, message.Reply(ctx.Event.MessageID))...)
			delay.Defer()
		}
		if response.Error != nil {
			errText := response.Error.Error()
			if strings.Contains(errText, "code=401") {
				// Token 过期了
				if args.TokenId != "" {
					if token := repo.GetToken(args.TokenId, "", ""); token != nil {
						token.Token = ""
						repo.UpdateToken(*token)
					}
				}
				deleteConversationContext(ctx)
			}
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(errText))
			delay.Close()
			return
		}

		if response.Status == xvars.Closed {
			logrus.Info("[MiaoX] - 结束应答")
			delay.Close()
		}
	}

	if e := lmt.Join(cctx, lmtHandle); e != nil {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(e.Error()))
	}
}

// AI作画
func drawCommand(ctx *zero.Ctx) {
	prompt := ctx.State["args"].(string)
	if prompt == "" {
		return
	}

	prompt = strings.ReplaceAll(prompt, "，", ",")
	global := repo.GetGlobal()
	if global.DrawServ == "" {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请先联系管理员设置AI作画API"))
		return
	}

	logrus.Info("接收到作画请求，开始作画：", prompt)
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("这就开始画 ~"))
	beginTime := time.Now()
	imgBytes, err := utils.DrawAI(global.DrawServ, prompt, global.DrawBody)
	if err != nil {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("作画失败："+err.Error()))
		return
	}

	seconds := time.Now().Sub(beginTime).Seconds()
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.ImageBytes(imgBytes), message.Text("耗时："+strconv.FormatFloat(seconds, 'f', 0, 64)+"s"))
}

// 添加凭证
func insertTokenCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	pattern := `^([^|]+)\:(.+)`
	r, _ := regexp.Compile(pattern)
	matches := r.FindStringSubmatch(value)
	logrus.Infoln(matches)
	if matches[1] == "" || matches[2] == "" {
		ctx.Send("添加失败，请按格式填写")
		return
	}
	global := repo.GetGlobal()
	billing, err := wapi.Query(context.Background(), matches[2], global.Proxy)
	if err != nil {
		logrus.Warn(err)
	}
	if billing.System-billing.Soft <= 0 {
		ctx.Send("添加失败，凭证余额为0")
		return
	}
	err = repo.InsertToken(repo.Token{
		Key:   matches[1],
		Token: matches[2],
		Type:  xvars.OpenAIAPI,
	})
	if err != nil {
		ctx.Send("添加失败: " + err.Error())
	} else {
		ctx.Send("添加成功，余额为" + strconv.FormatFloat(billing.System-billing.Soft, 'f', 2, 64))
	}
}

// 删除凭证
func deleteTokenCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	token := repo.GetToken("", value, "")
	if token == nil {
		ctx.Send("`" + value + "`不存在")
		return
	}
	repo.RemoveToken(value)
	ctx.Send("`" + value + "`已删除")
}

// 凭证列表
func tokensCommand(ctx *zero.Ctx) {
	doc := "凭证列表：\n"
	tokens, err := repo.FindTokens("")
	if err != nil {
		ctx.Send(doc + "None.")
		return
	}
	if len(tokens) <= 0 {
		ctx.Send(doc + "None.")
		return
	}
	for _, token := range tokens {
		doc += token.Type + " | " + token.Key + "\n"
	}
	ctx.Send(doc)
}

// 开启/切换预设
func enablePresetSceneCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.Send("获取上下文出错: " + err.Error())
		return
	}

	presetType := cctx.Bot
	if cctx.Bot == xvars.Claude && cctx.Model == clVars.Model4WebClaude2 {
		presetType = xvars.Claude + "-web"
	}

	presetScene := repo.GetPresetScene("", value, presetType)
	if presetScene == nil {
		ctx.Send("`" + value + "`预设不存在")
		return
	}

	if presetType != presetScene.Type {
		ctx.Send("当前AI类型无法使用`" + value + "`预设")
		return
	}

	cctx.Preset = presetScene.Content
	cctx.Format = presetScene.Message
	cctx.Chain = BaseChain + presetScene.Chain
	bot := cctx.Bot
	if bot == Poe {
		bot = xvars.OpenAIAPI
	}

	lmt.Remove(cctx.Id, bot)
	store.DeleteOnline(cctx.Id)
	updateConversationContext(cctx)
	ctx.Send("已切换`" + value + "`预设")
}

// 预设场景列表
func presetScenesCommand(ctx *zero.Ctx) {
	doc := "预设列表：\n"
	preset, err := repo.FindPresetScenes("")
	if err != nil {
		ctx.Send(doc + "None.")
		return
	}
	if len(preset) <= 0 {
		ctx.Send(doc + "None.")
		return
	}
	for _, token := range preset {
		doc += token.Type + " | " + token.Key + "\n"
	}
	ctx.Send(doc)
}

func switchAICommand(ctx *zero.Ctx) {
	bot := ctx.State["regex_matched"].([]string)[1]
	var cctx types.ConversationContext
	switch bot {
	case xvars.OpenAIAPI,
		xvars.OpenAIWeb,
		xvars.Claude,
		xvars.Claude + "-web",
		xvars.Bing + "-c",
		xvars.Bing + "-b",
		xvars.Bing + "-p",
		xvars.Bing + "-s",
		Poe + "-gpt3.5",
		Poe + "-gpt4",
		Poe + "-gpt4-32k",
		Poe + "-claude+",
		Poe + "-claude100k":
		deleteConversationContext(ctx)
		c, err := createConversationContext(ctx, bot)
		if err != nil {
			ctx.Send(err.Error())
			return
		}
		cctx = c
	default:
		ctx.Send("未知的AI类型：`" + bot + "`")
		return
	}

	lmt.Remove(cctx.Id, cctx.Bot)
	store.DeleteOnline(cctx.Id)
	ctx.Send("已切换`" + bot + "`AI模型")
}

func Run(addr string) {
	cmd.Run(addr)
	logrus.Info("已开启 `" + addr + "` Web服务")
}
