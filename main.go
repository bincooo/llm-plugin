package llm

import (
	"context"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/bincooo/AutoAI"
	"github.com/bincooo/edge-api/util"
	"github.com/bincooo/llm-plugin/cmd"
	"github.com/bincooo/llm-plugin/internal/chain"
	"github.com/bincooo/llm-plugin/internal/repo"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/bincooo/llm-plugin/internal/types"
	"github.com/bincooo/llm-plugin/utils"
	"github.com/bincooo/llm-plugin/vars"
	"github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/message"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"

	ctrl "github.com/FloatTech/zbpctrl"
	autostore "github.com/bincooo/AutoAI/store"
	autotypes "github.com/bincooo/AutoAI/types"
	xvars "github.com/bincooo/AutoAI/vars"
	claudevars "github.com/bincooo/claude-api/vars"
	wapi "github.com/bincooo/openai-wapi"
	zero "github.com/wdvxdr1123/ZeroBot"
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

	tts TTSMaker

	lmt autotypes.Limiter

	BB = []string{
		"太啰嗦了巴嘎 ♪(´ε｀ )",
		"这么长的文字你让我咋读啊 (*≧ω≦)",
		"太长了!! 受不了了~ (˶‾᷄ ⁻̫ ‾᷅˵)",
		"你是想把今天的话一次性说完嘛 ( ；´Д｀)",
		"简洁一点点吧，求求了 _(:_」∠)_",
		"要不你自己读读看拟写了什么 (╯‵□′)╯︵┻━┻",
	}
)

func init() {
	vars.E = engine

	var err error
	if vars.Loading, err = os.ReadFile(vars.E.DataFolder() + "/load.gif"); err != nil {
		panic(err)
	}

	// init tts
	tts.Reg("Edge", &_edgeTts{})

	lmt = AutoAI.NewCommonLimiter()
	if e := lmt.RegChain("tmpl", &chain.TplInterceptor{}); e != nil {
		panic(e)
	}
	if e := lmt.RegChain("online", &chain.OnlineInterceptor{}); e != nil {
		panic(e)
	}

	engine.OnRegex(`^添加凭证\s+(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(insertTokenCommand)
	engine.OnRegex(`^删除凭证\s+(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(deleteTokenCommand)
	engine.OnFullMatch("凭证列表", zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(tokensCommand)
	engine.OnRegex(`^切换凭证\s(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchTokensCommand)
	engine.OnRegex(`[开启|切换]预设\s(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchPresetSceneCommand)
	engine.OnRegex(`切换AI\s(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(switchAICommand)
	engine.OnFullMatch("预设列表", zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(presetScenesCommand)
	engine.OnFullMatch("历史对话", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(historyCommand)
	engine.OnRegex("语音列表", zero.OnlyToMe, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(ttsCommand)
	engine.OnRegex(`[开启|切换]语音\s(.+)`, zero.OnlyToMe, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchTTSCommand)
	engine.OnRegex(".+", zero.OnlyToMe, repo.OnceOnSuccess, excludeOnMessage).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(conversationCommand)

	cmd.Register("/api/global", repo.GlobalService{}, cmd.NewMenu("global", "全局配置"))
	cmd.Register("/api/preset", repo.PresetService{}, cmd.NewMenu("preset", "预设配置"))
	cmd.Register("/api/token", repo.TokenService{}, cmd.NewMenu("token", "凭证配置"))
}

// 自定义优先级
func excludeOnMessage(ctx *zero.Ctx) bool {
	msg := ctx.MessageString()
	exclude := []string{"添加凭证 ", "删除凭证 ", "凭证列表", "切换", "开启", "预设列表", "历史对话", "语音列表", "/", "!"}
	for _, value := range exclude {
		if strings.HasPrefix(msg, value) {
			return false
		}
	}
	return true
}

func historyCommand(ctx *zero.Ctx) {
	key := getId(ctx)
	messages := autostore.GetMessages(key)
	logrus.Info(messages)
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已在后台打印"))
}

// 语音列表
func ttsCommand(ctx *zero.Ctx) {
	ctx.SendChain(message.Text(tts.Echo()))
}

// 开启语音
func switchTTSCommand(ctx *zero.Ctx) {
	matched := ctx.State["regex_matched"].([]string)[1]
	index := strings.Index(matched, " ")
	if index <= 0 || len(matched)-1 == index {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("参数不正确: "+matched))
		return
	}

	key := strings.TrimSpace(matched[:index])
	value := strings.TrimSpace(matched[index:])
	if !tts.ContainTone(key, value) {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("不支持的语音类型: "+key+"/"+value))
		return
	}

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("发生异常: "+err.Error()))
		return
	}
	args := cctx.Data.(types.ConversationContextArgs)
	args.Tts = key + "/" + value
	cctx.Data = args
	updateConversationContext(cctx)
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("开启完毕"))
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

	prompt := parseMessage(ctx)
	// 限制对话长度
	str := []rune(prompt)
	if len(str) > 500 {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(BB[rand.Intn(len(BB))]))
		return
	}
	cctx.Prompt = prompt
	args := cctx.Data.(types.ConversationContextArgs)
	args.Current = strconv.FormatInt(ctx.Event.Sender.ID, 10)
	args.Nickname = ctx.Event.Sender.NickName
	cctx.Data = args
	// 使用了poe-openai-proxy
	if cctx.Bot == Poe {
		cctx.Bot = xvars.OpenAIAPI
	}

	delay := utils.NewDelay(ctx)
	lmtHandle := func(response autotypes.PartialResponse) {
		if response.Status == xvars.Begin {
			delay.Defer()
		}

		if len(response.Message) > 0 {
			if response.Status == xvars.Closed && strings.TrimSpace(response.Message) == "" {
			} else {
				if args.Tts == "" {
					segment := utils.StringToMessageSegment(cctx.Id, response.Message)
					ctx.SendChain(append(segment, message.Reply(ctx.Event.MessageID))...)
					delay.Defer()
				} else {
					// 使用线程，解除阻塞
					slice := strings.Split(args.Tts, "/")
					msg := strings.TrimSpace(response.Message)
					if msg == "" {
						goto label
					}
					go func() {
						audio, e := tts.Audio(slice[0], slice[1], response.Message)
						if e != nil {
							ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("生成语音失败："+e.Error()))
						} else {
							ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Record(audio))
						}
						delay.Defer()
					}()
				label:
				}
			}
		}

		if response.Error != nil {
			errText := response.Error.Error()
			go handleBingCaptcha(cctx.Token, response.Error)
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

// 切换AI凭证
func switchTokensCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.Send("获取上下文出错: " + err.Error())
		return
	}

	tokenType := cctx.Bot
	if cctx.Bot == xvars.Claude && cctx.Model == claudevars.Model4WebClaude2 {
		tokenType = xvars.Claude + "-web"
	}

	token := repo.GetToken("", value, tokenType)
	if token == nil {
		ctx.Send("`" + cctx.Bot + "`的`" + value + "`凭证不存在")
		return
	}

	if tokenType != token.Type {
		ctx.Send("当前AI类型无法使用`" + value + "`凭证")
		return
	}

	bot := cctx.Bot
	if bot == Poe {
		bot = xvars.OpenAIAPI
	}

	args := cctx.Data.(types.ConversationContextArgs)
	args.TokenId = token.Id
	cctx.Data = args
	cctx.Token = token.Token
	if token.BaseURL != "" {
		cctx.BaseURL = token.BaseURL
	}

	lmt.Remove(cctx.Id, bot)
	store.DeleteOnline(cctx.Id)
	updateConversationContext(cctx)
	ctx.Send("已切换`" + value + "`凭证")
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
func switchPresetSceneCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.Send("获取上下文出错: " + err.Error())
		return
	}

	presetType := cctx.Bot
	if cctx.Bot == xvars.Claude && cctx.Model == claudevars.Model4WebClaude2 {
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

	args := cctx.Data.(types.ConversationContextArgs)
	args.PresetId = presetScene.Id
	cctx.Data = args

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
	var cctx autotypes.ConversationContext
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

// 尝试解决Bing人机检验
func handleBingCaptcha(token string, err error) {
	content := err.Error()
	if strings.Contains(content, "User needs to solve CAPTCHA to continue") {
		// content = "用户需要人机验证...  已尝试自动验证，若重新生成文本无效请手动验证。"
		if strings.Contains(token, "_U=") {
			split := strings.Split(token, ";")
			for _, item := range split {
				if strings.Contains(item, "_U=") {
					token = strings.TrimSpace(strings.ReplaceAll(item, "_U=", ""))
					break
				}
			}
		}
		if e := util.SolveCaptcha(token); e != nil {
			logrus.Error("尝试解析Bing人机检验失败：", e)
		}
	}
}

func Run(addr string) {
	cmd.Run(addr)
	logrus.Info("已开启 `" + addr + "` Web服务")
}
