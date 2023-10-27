package llm

import (
	"encoding/json"
	"github.com/BurntSushi/toml"
	"github.com/FloatTech/NanoBot-Plugin/utils/ctxext"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	"github.com/bincooo/go-openai"
	"github.com/bincooo/llm-plugin/internal/chain"
	"github.com/bincooo/llm-plugin/internal/repo"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/bincooo/llm-plugin/internal/types"
	"github.com/bincooo/llm-plugin/internal/util"
	"github.com/bincooo/llm-plugin/internal/vars"
	nano "github.com/fumiama/NanoBot"
	"github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	adapter "github.com/bincooo/chatgpt-adapter"
	adstore "github.com/bincooo/chatgpt-adapter/store"
	adtypes "github.com/bincooo/chatgpt-adapter/types"
	advars "github.com/bincooo/chatgpt-adapter/vars"
	claudevars "github.com/bincooo/claude-api/vars"
)

const help = `- @Bot + 文本内容
- 昵称前缀 + 文本内容
- 预设列表
- [开启|切换]预设 + [预设名]
- 删除凭证 + [key]
- 添加凭证 + [key]:[value]
- 语音列表
- [开启|切换]语音 [type] [name]
- 关闭语音
- 切换AI + [AI类型：openai-api、openai-web、claude、、、]
- AI列表
`

var (
	engine = nano.Register("miaox", &ctrl.Options[*nano.Ctx]{
		Help:              help,
		Brief:             "喵小爱-AI适配器",
		DisableOnDefault:  false,
		PrivateDataFolder: "miaox",
	}).ApplySingle(ctxext.DefaultSingle)

	tts TTSMaker

	lmt adtypes.Limiter

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
	if !file.IsExist(vars.E.DataFolder() + "/load.gif") {
		data, e := web.GetData("https://cdn.jsdelivr.net/gh/bincooo/llm-plugin@data-1/load.gif")
		if e != nil {
			panic(e)
		}
		_ = os.WriteFile(vars.E.DataFolder()+"/load.gif", data, 0666)
	}
	if vars.Loading, err = os.ReadFile(vars.E.DataFolder() + "/load.gif"); err != nil {
		panic(err)
	}

	// init tts
	tts.Reg("Edge", &_edgeTts{})
	tts.Reg("genshin", &_genshinvoice{})

	lmt = adapter.NewCommonLimiter()
	// init chain
	if e := lmt.RegChain("tmpl", &chain.TplInterceptor{}); e != nil {
		panic(e)
	}
	if e := lmt.RegChain("online", &chain.OnlineInterceptor{}); e != nil {
		panic(e)
	}

	nano.OnMessageDelete(repo.OnceOnSuccess).SetBlock(false).Handle(recallMessageCommand)

	engine.OnMessageFullMatch("全局属性", nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(globalCommand)
	engine.OnMessageRegex(`^[添加|修改]全局属性\s+([\s\S]*)$`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(editGlobalCommand)
	engine.OnMessageRegex(`^[添加|修改]凭证\s+([\s\S]*)$`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(insertTokenCommand)
	engine.OnMessageRegex(`^删除凭证\s+(\S+)`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(deleteTokenCommand)
	engine.OnMessageFullMatch("凭证列表", nano.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(tokensCommand)
	engine.OnMessageRegex(`^凭证明细\s+(\S+)`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(tokenItemCommand)
	engine.OnMessageRegex(`^切换凭证\s+(\S+)`, nano.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchTokensCommand)
	engine.OnMessageFullMatch("AI列表", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(aiCommand)
	engine.OnMessageRegex(`切换AI\s+(\S+)`, nano.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchAICommand)
	engine.OnMessageRegex(`^[添加|修改]预设\s+([\s\S]*)$`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(insertRoleCommand)
	engine.OnMessageRegex(`^删除预设\s+(\S+)`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(deleteRoleCommand)
	engine.OnMessageFullMatch("预设列表", nano.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(rolesCommand)
	engine.OnMessageRegex(`^预设明细\s+(\S+)`, nano.AdminPermission, nano.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(roleItemCommand)
	engine.OnMessageRegex(`[开启|切换]预设\s(\S+)`, nano.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchRoleCommand)
	engine.OnMessageFullMatch("历史对话", nano.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(historyCommand)
	engine.OnMessageFullMatch("语音列表", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(ttsCommand)
	engine.OnMessageFullMatch("关闭语音", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(closeTTSCommand)
	engine.OnMessageRegex(`[开启|切换]语音\s(.+)`, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchTTSCommand)
	engine.OnMessage(nano.OnlyToMe, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(conversationCommand)
}

func globalCommand(ctx *nano.Ctx) {
	g := repo.GetGlobal()
	if _, err := ctx.SendPlainMessage(false, formatGlobal(g)); err != nil {
		logrus.Error(err)
	}
}

// 修改全局属性
func editGlobalCommand(ctx *nano.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	value = strings.ReplaceAll(value, "&#91;", "[")
	value = strings.ReplaceAll(value, "&#93;", "]")

	var g repo.GlobalConfig
	if _, err := toml.Decode(value, &g); err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "添加失败，请按格式填写："+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}

	if !validateBot(g.Bot) {
		if _, err := ctx.SendPlainMessage(false, "修改失败，AI类型不正确。可使用：[AI列表] 命令查看"); err != nil {
			logrus.Error(err)
		}
		return
	}

	dbg := repo.GetGlobal()
	if dbg != nil {
		// 等待用户下一步选择
		if index := waitCommand(ctx, time.Second*60, "已存在相同的全局属性，是否覆盖？", []string{"否", "是"}); index < 1 {
			if index == 0 {
				if _, err := ctx.SendPlainMessage(true, "已取消"); err != nil {
					logrus.Error(err)
				}
			}
			return
		}
		g.Id = dbg.Id
	}

	if err := repo.EditGlobal(g); err != nil {
		if _, err = ctx.SendPlainMessage(false, "修改失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
	} else {
		if _, err = ctx.SendPlainMessage(false, "修改成功"); err != nil {
			logrus.Error(err)
		}
	}
}

func aiCommand(ctx *nano.Ctx) {
	slice := map[string][]string{
		"openai": {
			"- openai-api (api接口)",
			"- openai-web (网页接口)",
		},
		"claude": {
			"- claude (slack接入)",
			"- claude-web (网页接入)",
		},
		"bing": {
			"- bing-c (创造性)",
			"- bing-b (平衡性)",
			"- bing-p (精确性)",
			"- bing-s (半解禁)",
		},
	}
	tex := ""
	for k, v := range slice {
		tex += "** " + k + " **\n" + strings.Join(v, "\n") + "\n\n"
	}
	if _, err := ctx.SendPlainMessage(false, nano.Text(tex)); err != nil {
		logrus.Error(err)
	}
}

func historyCommand(ctx *nano.Ctx) {
	key := getId(ctx)
	logrus.Info(adstore.GetMessages(key))
	if _, err := ctx.SendPlainMessage(false, nano.Text("已在后台打印")); err != nil {
		logrus.Error(err)
	}
}

// 语音列表
func ttsCommand(ctx *nano.Ctx) {
	if _, err := ctx.SendPlainMessage(false, tts.Echo()); err != nil {
		logrus.Error(err)
	}
}

// 关闭语音
func closeTTSCommand(ctx *nano.Ctx) {
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		if _, err = ctx.SendPlainMessage(false, nano.Text("发生异常: "+err.Error())); err != nil {
			logrus.Error(err)
		}
		return
	}
	args := cctx.Data.(types.ConversationContextArgs)
	args.Tts = ""
	cctx.Data = args
	updateConversationContext(cctx)
	if _, err = ctx.SendPlainMessage(false, nano.Text("关闭完毕")); err != nil {
		logrus.Error(err)
	}
}

// 开启语音
func switchTTSCommand(ctx *nano.Ctx) {
	matched := ctx.State["regex_matched"].([]string)[1]
	index := strings.Index(matched, " ")
	if index <= 0 || len(matched)-1 == index {
		if _, err := ctx.SendPlainMessage(false, nano.Text("参数不正确: "+matched)); err != nil {
			logrus.Error(err)
		}
		return
	}

	key := strings.TrimSpace(matched[:index])
	value := strings.TrimSpace(matched[index:])
	if !tts.ContainTone(key, value) {
		if _, err := ctx.SendPlainMessage(false, nano.Text("不支持的语音类型: "+key+"/"+value)); err != nil {
			logrus.Error(err)
		}
		return
	}

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		if _, err = ctx.SendPlainMessage(false, nano.Text("发生异常: "+err.Error())); err != nil {
			logrus.Error(err)
		}
		return
	}
	args := cctx.Data.(types.ConversationContextArgs)
	args.Tts = key + "/" + value
	cctx.Data = args
	updateConversationContext(cctx)
	if _, err = ctx.SendPlainMessage(false, nano.Text("开启完毕")); err != nil {
		logrus.Error(err)
	}
}

// 撤回消息时删除缓存中的消息记录
func recallMessageCommand(ctx *nano.Ctx) {
	delmsg := ctx.Value.(*nano.MessageDelete)
	adstore.DeleteMessageFor(getId(ctx), delmsg.Message.ID)
}

// 聊天
func conversationCommand(ctx *nano.Ctx) {
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(true, nano.Text(err)); err != nil {
			logrus.Error(err)
		}
		return
	}

	args := cctx.Data.(types.ConversationContextArgs)
	args.Nickname = ctx.Message.Member.Nick
	if ctx.Message.Member.User != nil {
		args.Current = ctx.Message.Member.User.ID
	}
	cctx.Data = args

	images := false
	if dbToken := repo.GetToken(args.Tid, "", ""); dbToken != nil {
		images = dbToken.Images == 1
	}

	prompt := parseMessage(ctx, images)
	messageId := ctx.Message.ID

	// 限制对话长度
	if len([]rune(prompt)) > 300 {
		if _, err = ctx.SendPlainMessage(true, nano.Text(BB[rand.Intn(len(BB))])); err != nil {
			logrus.Error(err)
		}
		return
	}

	cctx.Prompt = prompt
	cctx.MessageId = messageId

	section := false
	if role := repo.GetRole(args.Rid, "", ""); role != nil {
		section = role.Section == 1
	}

	// timer := util.NewGifTimer(ctx, section)
	cacheMessage := make([]string, 0)
	lmtHandle := func(response adtypes.PartialResponse) {
		//if response.Status == advars.Begin {
		//	timer.Refill()
		//}

		if len(strings.TrimSpace(response.Message)) > 0 {
			if section /* && args.Tts == "" */ {
				segment := util.StringToMessageSegment(cctx.Id, response.Message)
				if _, err = ctx.SendChain(append(segment, nano.ReplyTo(ctx.Message.ID))...); err != nil {
					logrus.Error(err)
				}
				// timer.Refill()
			} else {
				// 开启语音就不要用分段响应了
				cacheMessage = append(cacheMessage, response.Message)
			}
		}

		if response.Error != nil {
			logrus.Error(response.Error)
			go util.HandleBingCaptcha(cctx.Token, response.Error)
			if _, err = ctx.SendPlainMessage(true, nano.Text(response.Error)); err != nil {
				logrus.Error(err)
			}
			// timer.Release()
			return
		}

		if response.Status == advars.Closed {
			// 开启了语音
			//if args.Tts != "" {
			//	slice := strings.Split(args.Tts, "/")
			//	msg := strings.TrimSpace(strings.Join(cacheMessage, ""))
			//	if msg != "" {
			//		segment := util.StringToMessageSegment(cctx.Id, msg)
			//		if _, err = ctx.SendChain(append(segment, nano.ReplyTo(ctx.Message.ID))...); err != nil {
			//			logrus.Error(err)
			//		}
			//		audios, e := tts.Audio(slice[0], slice[1], msg)
			//		if e != nil {
			//			logrus.Error("生成语音失败：", e)
			//			if _, err = ctx.SendPlainMessage(true, e); err != nil {
			//				logrus.Error(err)
			//			}
			//		} else {
			//			for _, audio := range audios {
			//				time.Sleep(600 * time.Millisecond)
			//				ctx.SendChain(nano.Audio(audio))
			//			}
			//		}
			//	}
			//} else
			if !section { // 关闭了分段输出
				msg := strings.TrimSpace(strings.Join(cacheMessage, ""))
				if msg != "" {
					segment := util.StringToMessageSegment(cctx.Id, msg)
					if _, err = ctx.SendChain(append(segment, nano.ReplyTo(ctx.Message.ID))...); err != nil {
						logrus.Error(err)
					}
				}
			}
			logrus.Info("[MiaoX] - 结束应答")
			// timer.Release()
		}
	}

	if e := lmt.Join(cctx, lmtHandle); e != nil {
		if _, err = ctx.SendPlainMessage(true, e); err != nil {
			logrus.Error(err)
		}
	}
}

// 添加凭证
func insertTokenCommand(ctx *nano.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	value = strings.ReplaceAll(value, "&#91;", "[")
	value = strings.ReplaceAll(value, "&#93;", "]")

	var newToken repo.TokenConfig
	if _, err := toml.Decode(value, &newToken); err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "添加失败，请按格式填写："+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}

	if !validateBot(newToken.Type) {
		if _, err := ctx.SendPlainMessage(false, "修改失败，AI类型不正确。可使用：[AI列表] 命令查看"); err != nil {
			logrus.Error(err)
		}
		return
	}

	dbToken := repo.GetToken("", newToken.Key, newToken.Type)
	if dbToken != nil {
		// 等待用户下一步选择
		time.Sleep(3 * time.Second)
		if index := waitCommand(ctx, time.Second*60, "已存在相同的凭证，是否覆盖？", []string{"否", "是"}); index < 1 {
			if index == 0 {
				if _, err := ctx.SendPlainMessage(true, "已取消"); err != nil {
					logrus.Error(err)
				}
			}
			return
		}
		newToken.Id = dbToken.Id
	}

	if err := repo.EditToken(newToken); err != nil {
		if _, err = ctx.SendPlainMessage(false, "添加失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
	} else {
		if _, err = ctx.SendPlainMessage(false, "添加成功"); err != nil {
			logrus.Error(err)
		}
	}
}

// 删除凭证
func deleteTokenCommand(ctx *nano.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	tokens, err := repo.FindTokens(key, "")
	if err != nil {
		if _, err = ctx.SendPlainMessage(false, "删除失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}
	if len(tokens) == 0 {
		if _, err = ctx.SendPlainMessage(false, "`"+key+"`不存在"); err != nil {
			logrus.Error(err)
		}
		return
	}

	index := 0
	if len(tokens) > 1 {
		var cmd []string
		for _, dbToken := range tokens {
			cmd = append(cmd, dbToken.Key+"#"+dbToken.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要删除的凭证", cmd)
		if index == -1 {
			return
		}
	}

	repo.RemoveToken(tokens[index].Id)
	if _, err = ctx.SendPlainMessage(false, "`"+key+"#"+tokens[0].Type+"`已删除"); err != nil {
		logrus.Error(err)
	}
}

// 切换AI凭证
func switchTokensCommand(ctx *nano.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "获取上下文出错: "+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}

	bot := cctx.Bot
	if cctx.Bot == advars.Claude && cctx.Model == claudevars.Model4WebClaude2 {
		bot = advars.Claude + "-web"
	}

	token := repo.GetToken("", key, bot)
	if token == nil {
		if _, err = ctx.SendPlainMessage(false, "`"+cctx.Bot+"`的`"+key+"`凭证不存在"); err != nil {
			logrus.Error(err)
		}
		return
	}

	if bot != token.Type {
		if _, err = ctx.SendPlainMessage(false, "当前AI("+bot+")无法使用`"+key+"`凭证"); err != nil {
			logrus.Error(err)
		}
		return
	}

	args := cctx.Data.(types.ConversationContextArgs)
	cctx.Data = args
	cctx.Token = token.Token
	if token.BaseURL != "" {
		cctx.BaseURL = token.BaseURL
	}

	lmt.Remove(cctx.Id, bot)
	store.DeleteOnline(cctx.Id)
	updateConversationContext(cctx)
	if _, err = ctx.SendPlainMessage(false, "已切换`"+key+"`凭证"); err != nil {
		logrus.Error(err)
	}
}

// 凭证列表
func tokensCommand(ctx *nano.Ctx) {
	doc := "凭证列表：(bot|key)\n\n"
	tokens, err := repo.FindTokens("", "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, doc+"None."); err != nil {
			logrus.Error(err)
		}
		return
	}
	if len(tokens) <= 0 {
		if _, err = ctx.SendPlainMessage(false, doc+"None."); err != nil {
			logrus.Error(err)
		}
		return
	}
	for _, token := range tokens {
		doc += padding(token.Type, 20) + " | " + token.Key + "\n"
	}
	if _, err = ctx.SendPlainMessage(false, doc); err != nil {
		logrus.Error(err)
	}
}

// 凭证明细
func tokenItemCommand(ctx *nano.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	tokens, err := repo.FindTokens(key, "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "查询失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}
	if len(tokens) == 0 {
		if _, err = ctx.SendPlainMessage(false, "`"+key+"`不存在"); err != nil {
			logrus.Error(err)
		}
		return
	}

	index := 0
	if len(tokens) > 1 {
		var cmd []string
		for _, dbToken := range tokens {
			cmd = append(cmd, dbToken.Key+"#"+dbToken.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要查看的凭证", cmd)
		if index == -1 {
			return
		}
	}
	if _, err = ctx.SendPlainMessage(false, formatToken(tokens[index])); err != nil {
		logrus.Error(err)
	}
}

// 开启/切换预设
func switchRoleCommand(ctx *nano.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		if _, err = ctx.SendPlainMessage(false, "获取上下文出错: "+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}

	// ai类型
	t := cctx.Bot
	if cctx.Bot == advars.Claude && cctx.Model == claudevars.Model4WebClaude2 {
		t = advars.Claude + "-web"
	}

	role := repo.GetRole("", value, t)
	if role == nil {
		if _, err = ctx.SendPlainMessage(false, "`"+value+"`预设不存在"); err != nil {
			logrus.Error(err)
		}
		return
	}

	if t != role.Type {
		if _, err = ctx.SendPlainMessage(false, "当前AI类型无法使用`"+value+"`预设"); err != nil {
			logrus.Error(err)
		}
		return
	}

	args := cctx.Data.(types.ConversationContextArgs)
	args.Rid = role.Id
	cctx.Data = args

	cctx.Preset = role.Content
	cctx.Format = role.Message
	cctx.Chain = BaseChain + role.Chain
	bot := cctx.Bot

	lmt.Remove(cctx.Id, bot)
	store.DeleteOnline(cctx.Id)
	updateConversationContext(cctx)
	if _, err = ctx.SendPlainMessage(false, "已切换`"+value+"`预设"); err != nil {
		logrus.Error(err)
	}
}

// 添加预设
func insertRoleCommand(ctx *nano.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	value = strings.ReplaceAll(value, "&#91;", "[")
	value = strings.ReplaceAll(value, "&#93;", "]")
	value = strings.ReplaceAll(value, `\n`, "[!n]")
	value = strings.ReplaceAll(value, `\"`, "[!d]")

	var newRole repo.RoleConfig
	if _, err := toml.Decode(value, &newRole); err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "添加失败，请按格式填写："+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}

	if newRole.Content != "" {
		newRole.Content = strings.ReplaceAll(newRole.Content, "[!n]", `\n`)
		newRole.Content = strings.ReplaceAll(newRole.Content, "[!d]", `\"`)
	}

	if !validateType(newRole.Type) {
		if _, err := ctx.SendPlainMessage(false, "修改失败，AI类型不正确。请选择以下类型：\n - "+
			advars.OpenAIAPI+"\n - "+
			advars.OpenAIWeb+"\n - "+
			advars.Claude+"\n - "+
			advars.Claude+"-web\n - "+
			advars.Bing); err != nil {
			logrus.Error(err)
		}
		return
	}

	if newRole.Type == advars.OpenAIAPI && newRole.Content != "" {
		var preset []openai.ChatCompletionMessage
		if err := json.Unmarshal([]byte(newRole.Content), &preset); err != nil {
			logrus.Error("预设解析失败: ", err)
			if _, err = ctx.SendPlainMessage(false, "预设解析失败: "+err.Error()); err != nil {
				logrus.Error(err)
			}
			return
		}
	}

	dbRole := repo.GetRole("", newRole.Key, newRole.Type)
	if dbRole != nil {
		// 等待用户下一步选择
		if index := waitCommand(ctx, time.Second*60, "已存在相同的预设，是否覆盖？", []string{"否", "是"}); index < 1 {
			if index == 0 {
				if _, err := ctx.SendPlainMessage(true, nano.Text("已取消")); err != nil {
					logrus.Error(err)
				}
			}
			return
		}
		newRole.Id = dbRole.Id
	}

	if err := repo.EditRole(newRole); err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "添加失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
	} else {
		if _, err = ctx.SendPlainMessage(false, "添加成功"); err != nil {
			logrus.Error(err)
		}
	}
}

// 删除预设
func deleteRoleCommand(ctx *nano.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	roles, err := repo.FindRoles(key, "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "删除失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}
	if len(roles) == 0 {
		if _, err = ctx.SendPlainMessage(false, "`"+key+"`不存在"); err != nil {
			logrus.Error(err)
		}
		return
	}

	index := 0
	if len(roles) > 1 {
		var cmd []string
		for _, dbRole := range roles {
			cmd = append(cmd, dbRole.Key+"#"+dbRole.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要删除的凭证", cmd)
		if index == -1 {
			return
		}
	}

	repo.RemoveRole(roles[index].Id)
	if _, err = ctx.SendPlainMessage(false, "`"+key+"#"+roles[0].Type+"`已删除"); err != nil {
		logrus.Error(err)
	}
}

// 预设列表
func rolesCommand(ctx *nano.Ctx) {
	doc := "预设列表：\n"
	preset, err := repo.FindRoles("", "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, doc+"None."); err != nil {
			logrus.Error(err)
		}
		return
	}
	if len(preset) <= 0 {
		if _, err = ctx.SendPlainMessage(false, doc+"None."); err != nil {
			logrus.Error(err)
		}
		return
	}
	for _, token := range preset {
		doc += padding(token.Type, 20) + " | " + token.Key + "\n"
	}
	if _, err = ctx.SendPlainMessage(false, doc); err != nil {
		logrus.Error(err)
	}
}

// 预设明细
func roleItemCommand(ctx *nano.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	roles, err := repo.FindRoles(key, "")
	if err != nil {
		logrus.Error(err)
		if _, err = ctx.SendPlainMessage(false, "查询失败: "+err.Error()); err != nil {
			logrus.Error(err)
		}
		return
	}
	if len(roles) == 0 {
		if _, err = ctx.SendPlainMessage(false, "`"+key+"`不存在"); err != nil {
			logrus.Error(err)
		}
		return
	}

	index := 0
	if len(roles) > 1 {
		var cmd []string
		for _, dbRole := range roles {
			cmd = append(cmd, dbRole.Key+"#"+dbRole.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要查看的预设", cmd)
		if index == -1 {
			return
		}
	}

	if _, err = ctx.SendPlainMessage(false, formatRole(roles[index])); err != nil {
		logrus.Error(err)
	}
}

func switchAICommand(ctx *nano.Ctx) {
	bot := ctx.State["regex_matched"].([]string)[1]
	var cctx adtypes.ConversationContext
	switch bot {
	case advars.OpenAIAPI,
		advars.OpenAIWeb,
		advars.Claude,
		advars.Claude + "-web",
		advars.Bing + "-c",
		advars.Bing + "-b",
		advars.Bing + "-p",
		advars.Bing + "-s":
		deleteConversationContext(ctx)
		c, err := createConversationContext(ctx, bot)
		if err != nil {
			logrus.Error(err)
			if _, err = ctx.SendPlainMessage(false, err); err != nil {
				logrus.Error(err)
			}
			return
		}
		cctx = c
	default:
		if _, err := ctx.SendPlainMessage(false, "未知的AI类型：`"+bot+"`"); err != nil {
			logrus.Error(err)
		}
		return
	}

	lmt.Remove(cctx.Id, cctx.Bot)
	store.DeleteOnline(cctx.Id)
	if _, err := ctx.SendPlainMessage(false, "已切换`"+bot+"`AI模型"); err != nil {
		logrus.Error(err)
	}
}

// 等待下一步指令，返回指令下标，-1取消，index对应cmd
func waitCommand(ctx *nano.Ctx, timeout time.Duration, tips string, cmd []string) int {
	cmdtips := ""
	for i, c := range cmd {
		cmdtips += "[" + strconv.Itoa(i) + "] : " + c + "\n"
	}
	if _, err := ctx.SendPlainMessage(false, tips+"\n发送序号:\n"+cmdtips+" \n发送\"取消\"终止执行"); err != nil {
		logrus.Error(err)
		return -1
	}
	recv, cancel := nano.NewFutureEvent("message", 999, true, nano.RegexRule(`^(取消|\d+)$`), nano.CheckUser(ctx.Message.Author.ID)).Repeat()
	defer cancel()
	for {
		select {
		case <-time.After(timeout):
			if _, err := ctx.SendPlainMessage(false, "等待超时，已取消"); err != nil {
				logrus.Error(err)
			}
			return -1
		case r := <-recv:
			nextcmd := r.MessageString()
			if nextcmd == "取消" {
				if _, err := ctx.SendPlainMessage(false, "已取消"); err != nil {
					logrus.Error(err)
				}
				return -1
			}
			index, err := strconv.Atoi(nextcmd)
			if err != nil || index < 0 || index > len(cmd)-1 {
				if _, err = ctx.SendPlainMessage(false, "请输入正确的序号"); err != nil {
					logrus.Error(err)
					return -1
				}
				continue
			}
			return index
		}
	}
}

func padding(value string, num int) string {
	l := len(value) * 2
	if l < num {
		for i := 0; i < num-l; i++ {
			value += " "
		}
	}
	return value
}
